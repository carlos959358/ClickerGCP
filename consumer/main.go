package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
)

var (
	subscriber      *PubSubSubscriber
	subscriberReady = false
	updater         *FirestoreUpdater
	notifier        *BackendNotifier
)

func initializeSubscriber(ctx context.Context, projectID, subscriptionName, backendURL string) {
	log.Println("[Subscriber] Starting background initialization...")

	log.Println("[Subscriber] Initializing Firestore...")
	var err error
	updater, err = NewFirestoreUpdater(ctx, projectID)
	if err != nil {
		log.Printf("[Subscriber] ✗ Firestore initialization failed: %v", err)
		return
	}
	log.Println("[Subscriber] ✓ Firestore ready")

	log.Println("[Subscriber] Initializing backend notifier...")
	notifier = NewBackendNotifier(backendURL)
	log.Println("[Subscriber] ✓ Backend notifier ready")

	log.Println("[Subscriber] Creating Pub/Sub client...")
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("[Subscriber] ✗ Pub/Sub client creation failed: %v", err)
		return
	}
	defer pubsubClient.Close()
	log.Println("[Subscriber] ✓ Pub/Sub client ready")

	log.Printf("[Subscriber] Checking subscription: %s\n", subscriptionName)
	sub := pubsubClient.Subscription(subscriptionName)
	exists, err := sub.Exists(ctx)
	if err != nil {
		log.Printf("[Subscriber] ✗ Failed to check subscription: %v", err)
		return
	}
	if !exists {
		log.Printf("[Subscriber] ✗ Subscription %s does not exist", subscriptionName)
		return
	}
	log.Println("[Subscriber] ✓ Subscription verified")

	subscriber = NewPubSubSubscriber(sub, updater, notifier)
	subscriberReady = true

	// Start processing messages
	log.Println("[Subscriber] Starting message processing (max 10 concurrent)...")
	if err := subscriber.Start(ctx, 10); err != nil {
		log.Printf("[Subscriber] ✗ Subscriber error: %v", err)
		subscriberReady = false
	}
}

func main() {
	// Configuration from environment
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		log.Fatal("GCP_PROJECT_ID environment variable not set")
	}

	subscriptionName := os.Getenv("PUBSUB_SUBSCRIPTION")
	if subscriptionName == "" {
		subscriptionName = "click-consumer-sub"
	}

	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		log.Fatal("BACKEND_URL environment variable not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Consumer service starting on port %s", port)
	log.Printf("Project: %s, Subscription: %s, Backend: %s", projectID, subscriptionName, backendURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start subscriber initialization in background
	go initializeSubscriber(ctx, projectID, subscriptionName, backendURL)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if subscriberReady && subscriber != nil {
			msgs, errs := subscriber.GetStats()
			fmt.Fprintf(w, `{"status":"ready","messages_processed":%d,"errors":%d,"timestamp":%d}`,
				msgs, errs, time.Now().UTC().Unix())
		} else {
			fmt.Fprintf(w, `{"status":"initializing","timestamp":%d}`, time.Now().UTC().Unix())
		}
	})

	// Liveness probe endpoint
	http.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	// Pub/Sub push endpoint
	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed"}`))
			return
		}

		var pubsubMessage struct {
			Message struct {
				Data string `json:"data"`
				ID   string `json:"id"`
			} `json:"message"`
			Subscription string `json:"subscription"`
		}

		if err := json.NewDecoder(r.Body).Decode(&pubsubMessage); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"decode failed: %v"}`, err)
			return
		}

		// Process the message
		var event ClickEvent
		data, err := base64.StdEncoding.DecodeString(pubsubMessage.Message.Data)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"base64 failed: %v"}`, err)
			return
		}

		if err := json.Unmarshal(data, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"unmarshal failed: %v"}`, err)
			return
		}

		// Update Firestore
		if updater == nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"updater not initialized"}`))
			return
		}

		if err := updater.IncrementCounters(context.Background(), event.Country, event.Country); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"increment failed: %v"}`, err)
			return
		}

		// Get updated counters and notify backend
		counters, err := updater.GetCounters(context.Background())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"get failed: %v"}`, err)
			return
		}

		if notifier != nil {
			global := int64(0)
			if val, ok := counters["global"].(int64); ok {
				global = val
			}
			countries := make(map[string]interface{})
			if val, ok := counters["countries"].(map[string]interface{}); ok {
				countries = val
			}

			if err := notifier.NotifyCounterUpdate(global, countries); err != nil {
				log.Printf("Failed to notify backend: %v", err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start HTTP server for health checks
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      http.DefaultServeMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	log.Printf("Starting HTTP server on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Server error: %v", err)
	}
}
