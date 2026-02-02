package main

import "context"

// FirestoreUpdaterInterface defines the Firestore operations contract
type FirestoreUpdaterInterface interface {
	IncrementCounters(ctx context.Context, country, code string) error
	GetCounters(ctx context.Context) (map[string]interface{}, error)
	CheckIdempotency(ctx context.Context, messageID string) (bool, error)
	RecordProcessedMessage(ctx context.Context, messageID string, country string) error
	Close() error
}

// BackendNotifierInterface defines the backend notification contract
type BackendNotifierInterface interface {
	NotifyCounterUpdate(global int64, countries map[string]interface{}) error
}

// Ensure implementations conform to interfaces
var (
	_ FirestoreUpdaterInterface = (*FirestoreUpdater)(nil)
	_ BackendNotifierInterface  = (*BackendNotifier)(nil)
)
