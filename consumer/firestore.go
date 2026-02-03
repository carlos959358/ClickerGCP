package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreUpdater struct {
	client *firestore.Client
}

func NewFirestoreUpdater(ctx context.Context, projectID string) (*FirestoreUpdater, error) {
	databaseID := os.Getenv("FIRESTORE_DATABASE")
	if databaseID == "" {
		databaseID = "(default)"
	}
	log.Printf("[Firestore] Initializing client for project=%s, database=%s", projectID, databaseID)

	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		log.Printf("[Firestore] ERROR: Failed to create client: %v", err)
		return nil, fmt.Errorf("failed to create firestore client for database %s: %w", databaseID, err)
	}
	log.Printf("[Firestore] ✓ Client created successfully")

	return &FirestoreUpdater{
		client: client,
	}, nil
}

func (f *FirestoreUpdater) IncrementCounters(ctx context.Context, country, code string) error {
	log.Printf("[Firestore] IncrementCounters: country=%s, code=%s", country, code)

	// Start a transaction for atomic updates
	err := f.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		log.Printf("[Firestore] Transaction started for country=%s", code)

		// Increment global counter (use Set with MergeAll to create if doesn't exist)
		globalRef := f.client.Collection("counters").Doc("global")
		log.Printf("[Firestore] Updating global counter at path: %s", globalRef.Path)
		if err := tx.Set(globalRef, map[string]interface{}{
			"count": firestore.Increment(1),
		}, firestore.MergeAll); err != nil {
			log.Printf("[Firestore] ERROR: Failed to update global counter: %v", err)
			return fmt.Errorf("failed to update global counter: %w", err)
		}
		log.Printf("[Firestore] ✓ Global counter incremented")

		// Increment country counter (create if doesn't exist, otherwise merge)
		countryDocID := fmt.Sprintf("country_%s", code)
		countryRef := f.client.Collection("counters").Doc(countryDocID)
		log.Printf("[Firestore] Updating country counter at path: %s", countryRef.Path)
		if err := tx.Set(countryRef, map[string]interface{}{
			"country": country,
			"count":   firestore.Increment(1),
		}, firestore.MergeAll); err != nil {
			log.Printf("[Firestore] ERROR: Failed to update country counter for %s: %v", countryDocID, err)
			return fmt.Errorf("failed to update country counter: %w", err)
		}
		log.Printf("[Firestore] ✓ Country counter incremented for %s", countryDocID)

		return nil
	})

	if err != nil {
		log.Printf("[Firestore] ERROR: IncrementCounters transaction failed: %v", err)
		return err
	}
	log.Printf("[Firestore] ✓ IncrementCounters completed successfully for country=%s", country)
	return nil
}

func (f *FirestoreUpdater) GetCounters(ctx context.Context) (map[string]interface{}, error) {
	log.Printf("[Firestore] GetCounters: Starting to fetch all counters")
	result := make(map[string]interface{})

	// Get global counter
	log.Printf("[Firestore] Fetching global counter from counters/global")
	globalDoc, err := f.client.Collection("counters").Doc("global").Get(ctx)
	if err != nil {
		// If document doesn't exist, that's OK - just return 0
		if status.Code(err) == codes.NotFound {
			log.Printf("[Firestore] Global counter not found (NotFound), setting to 0")
			result["global"] = int64(0)
		} else {
			log.Printf("[Firestore] ERROR: Failed to get global counter: %v", err)
			return nil, fmt.Errorf("failed to get global counter: %w", err)
		}
	} else {
		globalCount := int64(0)
		if val, ok := globalDoc.Data()["count"]; ok {
			if c, ok := val.(int64); ok {
				globalCount = c
			}
		}
		log.Printf("[Firestore] ✓ Global counter retrieved: %d", globalCount)
		result["global"] = globalCount
	}

	// Get all country counters using snapshot
	log.Printf("[Firestore] Fetching all country counters from counters collection")
	countries := make(map[string]interface{})
	docs, err := f.client.Collection("counters").Documents(ctx).GetAll()
	if err != nil {
		log.Printf("[Firestore] ERROR: Failed to get counters: %v", err)
		return nil, fmt.Errorf("failed to get counters: %w", err)
	}

	log.Printf("[Firestore] Retrieved %d documents from counters collection", len(docs))
	for _, doc := range docs {
		docID := doc.Ref.ID
		if docID == "global" {
			continue
		}

		data := doc.Data()
		count := int64(0)
		if val, ok := data["count"]; ok {
			if c, ok := val.(int64); ok {
				count = c
			}
		}
		countryName, _ := data["country"].(string)
		log.Printf("[Firestore] Country counter: %s = %d (name: %s)", docID, count, countryName)

		countries[docID] = map[string]interface{}{
			"count":   count,
			"country": data["country"],
		}
	}

	result["countries"] = countries
	log.Printf("[Firestore] ✓ GetCounters completed: %d countries found", len(countries))
	return result, nil
}

// CheckIdempotency checks if a message has already been processed
func (f *FirestoreUpdater) CheckIdempotency(ctx context.Context, messageID string) (bool, error) {
	log.Printf("[Firestore] CheckIdempotency: Checking if messageID=%s was already processed", messageID)

	doc, err := f.client.Collection("processed_messages").Doc(messageID).Get(ctx)
	if err != nil {
		// If document doesn't exist, it hasn't been processed
		if status.Code(err) == codes.NotFound {
			log.Printf("[Firestore] ✓ Message %s not in processed_messages (new message)", messageID)
			return false, nil
		}
		log.Printf("[Firestore] ERROR: Failed to check idempotency for %s: %v", messageID, err)
		return false, fmt.Errorf("failed to check idempotency: %w", err)
	}

	exists := doc.Exists()
	if exists {
		log.Printf("[Firestore] WARN: Message %s already processed (idempotent)", messageID)
	} else {
		log.Printf("[Firestore] ✓ Message %s document exists but doesn't exist?", messageID)
	}
	return exists, nil
}

// RecordProcessedMessage records that a message has been successfully processed
func (f *FirestoreUpdater) RecordProcessedMessage(ctx context.Context, messageID string, country string) error {
	log.Printf("[Firestore] RecordProcessedMessage: Recording messageID=%s, country=%s", messageID, country)

	_, err := f.client.Collection("processed_messages").Doc(messageID).Set(ctx, map[string]interface{}{
		"messageId": messageID,
		"country":   country,
		"timestamp": time.Now().UTC(),
	})

	if err != nil {
		log.Printf("[Firestore] ERROR: Failed to record processed message %s: %v", messageID, err)
		return err
	}
	log.Printf("[Firestore] ✓ Processed message recorded: %s", messageID)
	return nil
}

func (f *FirestoreUpdater) Close() error {
	log.Printf("[Firestore] Closing Firestore client")
	if f.client != nil {
		err := f.client.Close()
		if err != nil {
			log.Printf("[Firestore] ERROR: Failed to close Firestore client: %v", err)
			return err
		}
		log.Printf("[Firestore] ✓ Firestore client closed")
	}
	return nil
}
