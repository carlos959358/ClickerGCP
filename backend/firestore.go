package main

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/firestore"
)

// FirestoreClient handles Firestore operations
type FirestoreClient struct {
	client *firestore.Client
}

// CounterData represents the counter data structure
type CounterData struct {
	Global    int64                  `json:"global"`
	Countries map[string]interface{} `json:"countries"`
}

// NewFirestoreClient creates a new Firestore client
func NewFirestoreClient(ctx context.Context, projectID string) (*FirestoreClient, error) {
	databaseID := os.Getenv("FIRESTORE_DATABASE")
	if databaseID == "" {
		databaseID = "(default)"
	}

	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client for database %s: %w", databaseID, err)
	}

	return &FirestoreClient{
		client: client,
	}, nil
}

// GetCounters retrieves the current counter values from Firestore
func (f *FirestoreClient) GetCounters(ctx context.Context) (*CounterData, error) {
	result := &CounterData{
		Countries: make(map[string]interface{}),
	}

	// Get global counter
	globalDoc, err := f.client.Collection("counters").Doc("global").Get(ctx)
	if err != nil {
		// Initialize if doesn't exist
		_, initErr := f.client.Collection("counters").Doc("global").Set(ctx, map[string]interface{}{
			"count": int64(0),
		})
		if initErr != nil {
			return nil, fmt.Errorf("failed to initialize global counter: %w", initErr)
		}
		result.Global = 0
	} else {
		globalCount := int64(0)
		if val, ok := globalDoc.Data()["count"]; ok {
			if c, ok := val.(int64); ok {
				globalCount = c
			} else if c, ok := val.(float64); ok {
				globalCount = int64(c)
			}
		}
		result.Global = globalCount
	}

	// Get all country counters
	iter := f.client.Collection("counters").Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err != nil {
			// Check if iterator is exhausted - this is normal
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

		result.Countries[docID] = map[string]interface{}{
			"count":   count,
			"country": data["country"],
		}
	}

	return result, nil
}

// Close closes the Firestore client
func (f *FirestoreClient) Close() error {
	if f.client != nil {
		return f.client.Close()
	}
	return nil
}
