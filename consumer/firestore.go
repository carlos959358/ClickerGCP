package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
)

type FirestoreUpdater struct {
	client *firestore.Client
}

func NewFirestoreUpdater(ctx context.Context, projectID string) (*FirestoreUpdater, error) {
	databaseID := os.Getenv("FIRESTORE_DATABASE")
	if databaseID == "" {
		databaseID = "(default)"
	}

	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client for database %s: %w", databaseID, err)
	}

	return &FirestoreUpdater{
		client: client,
	}, nil
}

func (f *FirestoreUpdater) IncrementCounters(ctx context.Context, country, code string) error {
	// Start a transaction for atomic updates
	err := f.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Increment global counter (use Set with MergeAll to create if doesn't exist)
		globalRef := f.client.Collection("counters").Doc("global")
		if err := tx.Set(globalRef, map[string]interface{}{
			"count": firestore.Increment(1),
		}, firestore.MergeAll); err != nil {
			return fmt.Errorf("failed to update global counter: %w", err)
		}

		// Increment country counter (create if doesn't exist, otherwise merge)
		countryRef := f.client.Collection("counters").Doc(fmt.Sprintf("country_%s", code))
		if err := tx.Set(countryRef, map[string]interface{}{
			"country": country,
			"count":   firestore.Increment(1),
		}, firestore.MergeAll); err != nil {
			return fmt.Errorf("failed to update country counter: %w", err)
		}

		return nil
	})

	return err
}

func (f *FirestoreUpdater) GetCounters(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Get global counter
	globalDoc, err := f.client.Collection("counters").Doc("global").Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get global counter: %w", err)
	}

	globalCount := int64(0)
	if val, ok := globalDoc.Data()["count"]; ok {
		if c, ok := val.(int64); ok {
			globalCount = c
		}
	}

	result["global"] = globalCount

	// Get all country counters
	countries := make(map[string]interface{})
	iter := f.client.Collection("counters").Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err != nil {
			// Check if iterator is exhausted - this is normal and means we're done
			if err.Error() == "no more items in iterator" || err.Error() == "iterator exhausted" {
				break
			}
			return nil, fmt.Errorf("failed to iterate counters: %w", err)
		}

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

		countries[docID] = map[string]interface{}{
			"count":   count,
			"country": data["country"],
		}
	}

	result["countries"] = countries
	return result, nil
}

// CheckIdempotency checks if a message has already been processed
func (f *FirestoreUpdater) CheckIdempotency(ctx context.Context, messageID string) (bool, error) {
	doc, err := f.client.Collection("processed_messages").Doc(messageID).Get(ctx)
	if err != nil {
		// If document doesn't exist, it hasn't been processed
		if err.Error() == "document not found" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check idempotency: %w", err)
	}
	return doc.Exists(), nil
}

// RecordProcessedMessage records that a message has been successfully processed
func (f *FirestoreUpdater) RecordProcessedMessage(ctx context.Context, messageID string, country string) error {
	_, err := f.client.Collection("processed_messages").Doc(messageID).Set(ctx, map[string]interface{}{
		"messageId": messageID,
		"country":   country,
		"timestamp": time.Now().UTC(),
	})
	return err
}

func (f *FirestoreUpdater) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}
