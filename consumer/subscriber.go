package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
)

type ClickEvent struct {
	Timestamp int64  `json:"timestamp"` // Unix timestamp in seconds
	Country   string `json:"country"`
	IP        string `json:"ip"`
}

type PubSubSubscriber struct {
	subscription *pubsub.Subscription
	updater      *FirestoreUpdater
	notifier     *BackendNotifier
	messageCount int64
	errorCount   int64
	mu           sync.RWMutex
}

func NewPubSubSubscriber(
	subscription *pubsub.Subscription,
	updater *FirestoreUpdater,
	notifier *BackendNotifier,
) *PubSubSubscriber {
	return &PubSubSubscriber{
		subscription: subscription,
		updater:      updater,
		notifier:     notifier,
	}
}

func (s *PubSubSubscriber) Start(ctx context.Context, maxConcurrency int) error {
	log.Printf("Starting Pub/Sub subscriber with max concurrency: %d", maxConcurrency)

	s.subscription.ReceiveSettings.MaxExtension = 10 * time.Minute
	s.subscription.ReceiveSettings.MaxOutstandingMessages = maxConcurrency
	s.subscription.ReceiveSettings.NumGoroutines = maxConcurrency

	// Log stats periodically
	go s.logStats()

	return s.subscription.Receive(ctx, s.handleMessage)
}

func (s *PubSubSubscriber) handleMessage(ctx context.Context, msg *pubsub.Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in message handler: %v", r)
			msg.Nack()
		}
	}()

	// Parse click event
	var event ClickEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		atomic.AddInt64(&s.errorCount, 1)
		msg.Ack()
		return
	}

	log.Printf("Processing click: country=%s, ip=%s", event.Country, event.IP)

	// Update Firestore
	if err := s.updater.IncrementCounters(ctx, event.Country, event.Country); err != nil {
		log.Printf("Failed to update counters: %v", err)
		atomic.AddInt64(&s.errorCount, 1)
		msg.Nack()
		return
	}

	// Fetch updated counters
	counters, err := s.updater.GetCounters(ctx)
	if err != nil {
		log.Printf("Failed to get counters: %v", err)
		atomic.AddInt64(&s.errorCount, 1)
		msg.Nack()
		return
	}

	// Extract global and countries for notification
	global := int64(0)
	if val, ok := counters["global"].(int64); ok {
		global = val
	}

	countries := make(map[string]interface{})
	if val, ok := counters["countries"].(map[string]interface{}); ok {
		countries = val
	}

	// Notify backend
	if err := s.notifier.NotifyCounterUpdate(global, countries); err != nil {
		log.Printf("Failed to notify backend: %v", err)
		atomic.AddInt64(&s.errorCount, 1)
		// Still ack the message since we updated Firestore successfully
	}

	atomic.AddInt64(&s.messageCount, 1)
	msg.Ack()
}

func (s *PubSubSubscriber) logStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		msgCount := atomic.LoadInt64(&s.messageCount)
		errCount := atomic.LoadInt64(&s.errorCount)
		log.Printf("Stats - Messages: %d, Errors: %d", msgCount, errCount)
	}
}

func (s *PubSubSubscriber) GetStats() (messages, errors int64) {
	return atomic.LoadInt64(&s.messageCount), atomic.LoadInt64(&s.errorCount)
}
