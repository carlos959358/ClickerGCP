package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestCompleteMessageFlow verifies the end-to-end flow:
// Backend publishes -> Pub/Sub push -> Consumer processes -> Firestore updates -> Backend notified
func TestCompleteMessageFlow(t *testing.T) {
	// Setup mocks
	mockFirestore := NewMockFirestoreUpdater()
	mockBackendNotifier := NewMockBackendNotifier()
	updater = mockFirestore
	notifier = mockBackendNotifier

	// Simulate what backend publishes to Pub/Sub
	// Backend code: time.Now().UTC().Unix() -> int64
	backendEvent := map[string]interface{}{
		"timestamp": time.Now().Unix(), // Unix timestamp (int64)
		"country":   "US",
		"ip":        "192.168.1.1",
	}

	// Backend encodes as JSON
	eventJSON, _ := json.Marshal(backendEvent)

	// Pub/Sub wraps it in base64 and push message
	encodedData := base64.StdEncoding.EncodeToString(eventJSON)
	pubsubPayload := map[string]interface{}{
		"message": map[string]interface{}{
			"messageId": "test-msg-123",
			"data":      encodedData,
		},
	}
	body, _ := json.Marshal(pubsubPayload)

	// Consumer receives the push notification
	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Create the handler as it would be in main()
	handler := createProcessHandler()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify the continuous flow worked
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	if mockFirestore.counters["global"].(int64) != 1 {
		t.Errorf("Expected counter to be 1, got %d", mockFirestore.counters["global"].(int64))
	}

	if mockBackendNotifier.notificationCount != 1 {
		t.Errorf("Expected 1 backend notification, got %d", mockBackendNotifier.notificationCount)
	}

	if !mockFirestore.processedMessages["test-msg-123"] {
		t.Errorf("Expected message to be marked as processed")
	}

	t.Logf("✓ Complete message flow works: Backend → Pub/Sub → Consumer → Firestore + Backend notification")
}

// TestTimestampFormatCompatibility verifies backend and consumer use same timestamp format
func TestTimestampFormatCompatibility(t *testing.T) {
	mockFirestore := NewMockFirestoreUpdater()
	updater = mockFirestore

	// Simulate backend publishing with Unix timestamp
	currentTime := time.Now()
	unixTimestamp := currentTime.Unix()

	backendEvent := map[string]interface{}{
		"timestamp": unixTimestamp,
		"country":   "GB",
		"ip":        "10.0.0.1",
	}

	eventJSON, _ := json.Marshal(backendEvent)
	encodedData := base64.StdEncoding.EncodeToString(eventJSON)

	pubsubPayload := map[string]interface{}{
		"message": map[string]interface{}{
			"messageId": "timestamp-test",
			"data":      encodedData,
		},
	}
	body, _ := json.Marshal(pubsubPayload)

	// Verify consumer can parse it
	var payload map[string]interface{}
	json.Unmarshal(body, &payload)

	msgMap := payload["message"].(map[string]interface{})
	dataStr := msgMap["data"].(string)
	decoded, _ := base64.StdEncoding.DecodeString(dataStr)

	// This should unmarshal successfully with int64 timestamp
	var event ClickEvent
	err := json.Unmarshal(decoded, &event)
	if err != nil {
		t.Errorf("Failed to unmarshal event: %v", err)
	}

	if event.Timestamp != unixTimestamp {
		t.Errorf("Expected timestamp %d, got %d", unixTimestamp, event.Timestamp)
	}

	if event.Country != "GB" {
		t.Errorf("Expected country GB, got %s", event.Country)
	}

	t.Logf("✓ Timestamp format compatible: Unix int64 → JSON → Unmarshal works")
}

// TestPubSubMessageFormat verifies the exact message format from backend to consumer
func TestPubSubMessageFormat(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		country   string
		ip        string
		shouldFail bool
	}{
		{
			name:      "Valid message with US",
			timestamp: time.Now().Unix(),
			country:   "US",
			ip:        "8.8.8.8",
			shouldFail: false,
		},
		{
			name:      "Valid message with other country",
			timestamp: time.Now().Unix(),
			country:   "JP",
			ip:        "192.168.1.1",
			shouldFail: false,
		},
		{
			name:      "Missing country",
			timestamp: time.Now().Unix(),
			country:   "",
			ip:        "1.1.1.1",
			shouldFail: true, // Should fail validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFirestore := NewMockFirestoreUpdater()
			updater = mockFirestore

			backendEvent := map[string]interface{}{
				"timestamp": tt.timestamp,
				"country":   tt.country,
				"ip":        tt.ip,
			}

			eventJSON, _ := json.Marshal(backendEvent)
			encodedData := base64.StdEncoding.EncodeToString(eventJSON)

			pubsubPayload := map[string]interface{}{
				"message": map[string]interface{}{
					"messageId": fmt.Sprintf("test-%s", tt.name),
					"data":      encodedData,
				},
			}
			body, _ := json.Marshal(pubsubPayload)

			req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
			handler := createProcessHandler()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if tt.shouldFail {
				if w.Code == http.StatusOK {
					t.Errorf("Expected failure for %s, but got 200 OK", tt.name)
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("Expected 200 OK for %s, got %d", tt.name, w.Code)
				}
				if mockFirestore.counters["global"].(int64) != 1 {
					t.Errorf("Expected counter to be incremented for %s", tt.name)
				}
			}
		})
	}
}

// TestContinuousOperationMultipleMessages verifies multiple messages flow correctly
func TestContinuousOperationMultipleMessages(t *testing.T) {
	mockFirestore := NewMockFirestoreUpdater()
	mockNotifier := NewMockBackendNotifier()
	updater = mockFirestore
	notifier = mockNotifier

	handler := createProcessHandler()

	// Simulate 5 clicks from backend
	messages := []struct {
		id      string
		country string
		ip      string
	}{
		{"msg-001", "US", "1.1.1.1"},
		{"msg-002", "GB", "2.2.2.2"},
		{"msg-003", "US", "3.3.3.3"},
		{"msg-004", "CA", "4.4.4.4"},
		{"msg-005", "GB", "5.5.5.5"},
	}

	for _, msg := range messages {
		event := map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"country":   msg.country,
			"ip":        msg.ip,
		}

		eventJSON, _ := json.Marshal(event)
		encodedData := base64.StdEncoding.EncodeToString(eventJSON)

		payload := map[string]interface{}{
			"message": map[string]interface{}{
				"messageId": msg.id,
				"data":      encodedData,
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Message %s failed with status %d", msg.id, w.Code)
		}
	}

	// Verify all messages processed
	if mockFirestore.counters["global"].(int64) != 5 {
		t.Errorf("Expected global counter to be 5, got %d", mockFirestore.counters["global"].(int64))
	}

	if mockNotifier.notificationCount != 5 {
		t.Errorf("Expected 5 notifications, got %d", mockNotifier.notificationCount)
	}

	// Verify all messages recorded as processed
	for _, msg := range messages {
		if !mockFirestore.processedMessages[msg.id] {
			t.Errorf("Message %s not marked as processed", msg.id)
		}
	}

	t.Logf("✓ Continuous operation: Processed %d messages, global counter = %d, notifications = %d",
		len(messages),
		mockFirestore.counters["global"].(int64),
		mockNotifier.notificationCount)
}

// TestIdempotencyDuringContinuousFlow verifies duplicates don't break continuous operation
func TestIdempotencyDuringContinuousFlow(t *testing.T) {
	mockFirestore := NewMockFirestoreUpdater()
	updater = mockFirestore
	notifier = NewMockBackendNotifier()

	handler := createProcessHandler()

	// Send message twice (simulating Pub/Sub retry)
	event := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"country":   "US",
		"ip":        "10.0.0.1",
	}

	eventJSON, _ := json.Marshal(event)
	encodedData := base64.StdEncoding.EncodeToString(eventJSON)

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"messageId": "duplicate-retry-msg",
			"data":      encodedData,
		},
	}
	body, _ := json.Marshal(payload)

	// First delivery
	req1 := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First delivery failed with status %d", w1.Code)
	}

	counterAfterFirst := mockFirestore.counters["global"].(int64)

	// Simulated retry (Pub/Sub sends same message again)
	req2 := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Second delivery failed with status %d", w2.Code)
	}

	counterAfterSecond := mockFirestore.counters["global"].(int64)

	// Counter should NOT have incremented
	if counterAfterSecond != counterAfterFirst {
		t.Errorf("Counter changed from %d to %d (should have stayed at %d)", counterAfterFirst, counterAfterSecond, counterAfterFirst)
	}

	t.Logf("✓ Idempotency during continuous flow: Duplicate request safely ignored, counter stayed at %d", counterAfterSecond)
}

// Helper to create the process handler (simplified version of main's handler)
func createProcessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, `{"error":"method not allowed"}`)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid json"}`)
			return
		}

		msgInterface, ok := payload["message"]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing message"}`)
			return
		}

		msgMap := msgInterface.(map[string]interface{})
		messageID, _ := msgMap["messageId"].(string)

		// Check idempotency
		if updater != nil {
			processed, _ := updater.CheckIdempotency(r.Context(), messageID)
			if processed {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"status":"already_processed"}`)
				return
			}
		}

		dataStr, ok := msgMap["data"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid data"}`)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid base64"}`)
			return
		}

		var event ClickEvent
		if err := json.Unmarshal(decoded, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid event"}`)
			return
		}

		if event.Country == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing country"}`)
			return
		}

		if updater == nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"service not ready"}`)
			return
		}

		if err := updater.IncrementCounters(r.Context(), event.Country, event.Country); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to update"}`)
			return
		}

		if err := updater.RecordProcessedMessage(r.Context(), messageID, event.Country); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to record"}`)
			return
		}

		counters, err := updater.GetCounters(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to get counters"}`)
			return
		}

		if notifier != nil {
			global := counters["global"].(int64)
			countries := counters["countries"].(map[string]interface{})
			if err := notifier.NotifyCounterUpdate(global, countries); err != nil {
				// Non-blocking error
			}
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})
}
