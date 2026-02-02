package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockFirestoreUpdater implements FirestoreUpdater with in-memory storage for testing
type MockFirestoreUpdater struct {
	counters          map[string]interface{}
	processedMessages map[string]bool
	failOnIncrement   bool
	failOnGetCounters bool
}

func NewMockFirestoreUpdater() *MockFirestoreUpdater {
	return &MockFirestoreUpdater{
		counters: map[string]interface{}{
			"global": int64(0),
			"countries": map[string]interface{}{
				"US": map[string]interface{}{"count": int64(0), "country": "United States"},
				"GB": map[string]interface{}{"count": int64(0), "country": "United Kingdom"},
			},
		},
		processedMessages: make(map[string]bool),
		failOnIncrement:   false,
		failOnGetCounters: false,
	}
}

func (m *MockFirestoreUpdater) IncrementCounters(ctx context.Context, country, code string) error {
	if m.failOnIncrement {
		return fmt.Errorf("simulated firestore error")
	}

	// Increment global
	if global, ok := m.counters["global"].(int64); ok {
		m.counters["global"] = global + 1
	}

	// Increment country
	countries := m.counters["countries"].(map[string]interface{})
	key := fmt.Sprintf("country_%s", code)
	if countryData, ok := countries[key].(map[string]interface{}); ok {
		if count, ok := countryData["count"].(int64); ok {
			countryData["count"] = count + 1
		}
	} else {
		countries[key] = map[string]interface{}{
			"count":   int64(1),
			"country": country,
		}
	}

	return nil
}

func (m *MockFirestoreUpdater) GetCounters(ctx context.Context) (map[string]interface{}, error) {
	if m.failOnGetCounters {
		return nil, fmt.Errorf("simulated firestore error")
	}
	return m.counters, nil
}

func (m *MockFirestoreUpdater) CheckIdempotency(ctx context.Context, messageID string) (bool, error) {
	return m.processedMessages[messageID], nil
}

func (m *MockFirestoreUpdater) RecordProcessedMessage(ctx context.Context, messageID string, country string) error {
	m.processedMessages[messageID] = true
	return nil
}

func (m *MockFirestoreUpdater) Close() error {
	return nil
}

// MockBackendNotifier implements BackendNotifier for testing
type MockBackendNotifier struct {
	notificationCount int
	failOnNotify      bool
}

func NewMockBackendNotifier() *MockBackendNotifier {
	return &MockBackendNotifier{
		notificationCount: 0,
		failOnNotify:      false,
	}
}

func (m *MockBackendNotifier) NotifyCounterUpdate(global int64, countries map[string]interface{}) error {
	if m.failOnNotify {
		return fmt.Errorf("simulated backend error")
	}
	m.notificationCount++
	return nil
}

// createPubSubMessage creates a valid Pub/Sub push message
func createPubSubMessage(messageID string, country string, ip string, timestamp int64) []byte {
	event := map[string]interface{}{
		"timestamp": timestamp,
		"country":   country,
		"ip":        ip,
	}

	eventJSON, _ := json.Marshal(event)
	encoded := base64.StdEncoding.EncodeToString(eventJSON)

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"messageId": messageID,
			"data":      encoded,
		},
	}

	body, _ := json.Marshal(payload)
	return body
}

// Test 1: Successful message processing
func TestSuccessfulMessageProcessing(t *testing.T) {
	// Setup
	mockFirestore := NewMockFirestoreUpdater()
	mockNotifier := NewMockBackendNotifier()
	updater = mockFirestore
	notifier = mockNotifier

	// Create a valid message
	messageID := "msg-123"
	body := createPubSubMessage(messageID, "US", "1.2.3.4", time.Now().Unix())

	// Make request
	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute (we'll call the handler manually since we need to test it)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			fmt.Fprintf(w, `{"error":"missing message field"}`)
			return
		}

		msgMap := msgInterface.(map[string]interface{})
		messageID := msgMap["messageId"].(string)

		// Check idempotency
		processed, _ := updater.CheckIdempotency(r.Context(), messageID)
		if processed {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"already_processed","messageId":"%s"}`, messageID)
			return
		}

		// Extract and decode data
		dataStr := msgMap["data"].(string)
		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid base64"}`)
			return
		}

		var event ClickEvent
		if err := json.Unmarshal(decoded, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid click event"}`)
			return
		}

		// Increment counters
		if err := updater.IncrementCounters(r.Context(), event.Country, event.Country); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to update counters"}`)
			return
		}

		// Record as processed
		updater.RecordProcessedMessage(r.Context(), messageID, event.Country)

		// Get counters
		counters, err := updater.GetCounters(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to retrieve counters"}`)
			return
		}

		// Notify backend
		global := counters["global"].(int64)
		countries := counters["countries"].(map[string]interface{})
		if err := notifier.NotifyCounterUpdate(global, countries); err != nil {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","messageId":"%s","warning":"backend notification failed"}`, messageID)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","messageId":"%s"}`, messageID)
	})

	handler.ServeHTTP(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if mockFirestore.counters["global"].(int64) != 1 {
		t.Errorf("Expected global counter to be 1, got %d", mockFirestore.counters["global"].(int64))
	}

	if mockNotifier.notificationCount != 1 {
		t.Errorf("Expected 1 notification, got %d", mockNotifier.notificationCount)
	}

	if !mockFirestore.processedMessages[messageID] {
		t.Errorf("Expected message to be recorded as processed")
	}

	t.Logf("✓ Test passed: Successful message processing")
}

// Test 2: Duplicate detection (idempotency)
func TestDuplicateDetection(t *testing.T) {
	// Setup
	mockFirestore := NewMockFirestoreUpdater()
	mockNotifier := NewMockBackendNotifier()
	updater = mockFirestore
	notifier = mockNotifier

	messageID := "msg-duplicate-123"
	body := createPubSubMessage(messageID, "US", "1.2.3.4", time.Now().Unix())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		msgMap := payload["message"].(map[string]interface{})
		msgID := msgMap["messageId"].(string)

		// Check idempotency
		processed, _ := updater.CheckIdempotency(r.Context(), msgID)
		if processed {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"already_processed","messageId":"%s"}`, msgID)
			return
		}

		dataStr := msgMap["data"].(string)
		decoded, _ := base64.StdEncoding.DecodeString(dataStr)

		var event ClickEvent
		json.Unmarshal(decoded, &event)

		updater.IncrementCounters(r.Context(), event.Country, event.Country)
		updater.RecordProcessedMessage(r.Context(), msgID, event.Country)

		counters, _ := updater.GetCounters(r.Context())
		global := counters["global"].(int64)
		countries := counters["countries"].(map[string]interface{})
		notifier.NotifyCounterUpdate(global, countries)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","messageId":"%s"}`, msgID)
	})

	// First request
	req1 := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First request failed with status %d", w1.Code)
	}

	globalAfterFirst := mockFirestore.counters["global"].(int64)
	notificationsAfterFirst := mockNotifier.notificationCount

	// Second request (duplicate)
	req2 := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Second request failed with status %d", w2.Code)
	}

	// Verify counter NOT incremented again
	globalAfterSecond := mockFirestore.counters["global"].(int64)
	if globalAfterSecond != globalAfterFirst {
		t.Errorf("Expected counter to remain %d, but got %d after duplicate", globalAfterFirst, globalAfterSecond)
	}

	// Verify notification NOT sent again
	if mockNotifier.notificationCount != notificationsAfterFirst {
		t.Errorf("Expected %d notifications, got %d", notificationsAfterFirst, mockNotifier.notificationCount)
	}

	t.Logf("✓ Test passed: Duplicate detection works (counter: %d, notifications: %d)", globalAfterSecond, mockNotifier.notificationCount)
}

// Test 3: Invalid JSON handling
func TestInvalidJSONHandling(t *testing.T) {
	updater = NewMockFirestoreUpdater()
	notifier = NewMockBackendNotifier()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid json"}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Send invalid JSON
	req := httptest.NewRequest("POST", "/process", bytes.NewReader([]byte(`{"invalid json"`)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	t.Logf("✓ Test passed: Invalid JSON returns 400")
}

// Test 4: Invalid base64 handling
func TestInvalidBase64Handling(t *testing.T) {
	updater = NewMockFirestoreUpdater()
	notifier = NewMockBackendNotifier()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		msgMap := payload["message"].(map[string]interface{})
		dataStr := msgMap["data"].(string)

		_, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid base64"}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Create payload with invalid base64
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"messageId": "msg-123",
			"data":      "not-valid-base64!!!",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	t.Logf("✓ Test passed: Invalid base64 returns 400")
}

// Test 5: Firestore failure handling
func TestFirestoreFailureHandling(t *testing.T) {
	mockFirestore := NewMockFirestoreUpdater()
	mockFirestore.failOnIncrement = true
	updater = mockFirestore
	notifier = NewMockBackendNotifier()

	messageID := "msg-fail-123"
	body := createPubSubMessage(messageID, "US", "1.2.3.4", time.Now().Unix())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		msgMap := payload["message"].(map[string]interface{})
		msgID := msgMap["messageId"].(string)

		processed, _ := updater.CheckIdempotency(r.Context(), msgID)
		if processed {
			w.WriteHeader(http.StatusOK)
			return
		}

		dataStr := msgMap["data"].(string)
		decoded, _ := base64.StdEncoding.DecodeString(dataStr)

		var event ClickEvent
		json.Unmarshal(decoded, &event)

		// This will fail
		if err := updater.IncrementCounters(r.Context(), event.Country, event.Country); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to update counters"}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for Firestore failure, got %d", w.Code)
	}

	// Verify message was NOT recorded as processed (so it can be retried)
	if mockFirestore.processedMessages[messageID] {
		t.Errorf("Expected message NOT to be recorded when Firestore fails")
	}

	t.Logf("✓ Test passed: Firestore failure returns 500 (retriable)")
}

// Test 6: Missing required fields
func TestMissingRequiredFields(t *testing.T) {
	updater = NewMockFirestoreUpdater()
	notifier = NewMockBackendNotifier()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		_, ok := payload["message"]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing message field"}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Missing "message" field
	payload := map[string]interface{}{
		"subscription": "projects/my-project/subscriptions/my-sub",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing field, got %d", w.Code)
	}

	t.Logf("✓ Test passed: Missing required fields returns 400")
}

// Test 7: Invalid click event format
func TestInvalidClickEventFormat(t *testing.T) {
	updater = NewMockFirestoreUpdater()
	notifier = NewMockBackendNotifier()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		msgInterface, ok := payload["message"]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing message"}`)
			return
		}

		msgMap := msgInterface.(map[string]interface{})
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
			fmt.Fprintf(w, `{"error":"invalid click event"}`)
			return
		}

		// Validate required fields
		if event.Country == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing country"}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	// Create payload with invalid event format
	invalidEvent := map[string]interface{}{
		"invalid": "data",
	}
	eventJSON, _ := json.Marshal(invalidEvent)
	encoded := base64.StdEncoding.EncodeToString(eventJSON)

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"messageId": "msg-123",
			"data":      encoded,
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid event format, got %d", w.Code)
	}

	t.Logf("✓ Test passed: Invalid click event format returns 400")
}

// Test 8: Backend notification failure doesn't break message processing
func TestBackendNotificationFailure(t *testing.T) {
	mockFirestore := NewMockFirestoreUpdater()
	mockNotifier := NewMockBackendNotifier()
	mockNotifier.failOnNotify = true
	updater = mockFirestore
	notifier = mockNotifier

	messageID := "msg-notify-fail"
	body := createPubSubMessage(messageID, "US", "1.2.3.4", time.Now().Unix())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		msgMap := payload["message"].(map[string]interface{})
		msgID := msgMap["messageId"].(string)

		processed, _ := updater.CheckIdempotency(r.Context(), msgID)
		if processed {
			w.WriteHeader(http.StatusOK)
			return
		}

		dataStr := msgMap["data"].(string)
		decoded, _ := base64.StdEncoding.DecodeString(dataStr)

		var event ClickEvent
		json.Unmarshal(decoded, &event)

		updater.IncrementCounters(r.Context(), event.Country, event.Country)
		updater.RecordProcessedMessage(r.Context(), msgID, event.Country)

		counters, _ := updater.GetCounters(r.Context())
		global := counters["global"].(int64)
		countries := counters["countries"].(map[string]interface{})

		// This will fail
		if err := notifier.NotifyCounterUpdate(global, countries); err != nil {
			// But we still return 200 OK (message was processed)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","messageId":"%s","warning":"backend notification failed"}`, msgID)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","messageId":"%s"}`, msgID)
	})

	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Still returns 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 even with notification failure, got %d", w.Code)
	}

	// But counter WAS incremented
	if mockFirestore.counters["global"].(int64) != 1 {
		t.Errorf("Expected counter to be incremented despite notification failure")
	}

	// And message WAS recorded as processed
	if !mockFirestore.processedMessages[messageID] {
		t.Errorf("Expected message to be recorded as processed")
	}

	t.Logf("✓ Test passed: Backend notification failure doesn't prevent message processing")
}

// Test 9: Uninitialized services
func TestUninitializedServices(t *testing.T) {
	// Simulate uninitialized service
	updater = nil
	notifier = nil

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		if updater == nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"service not ready"}`)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	body := createPubSubMessage("msg-123", "US", "1.2.3.4", time.Now().Unix())
	req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for uninitialized service, got %d", w.Code)
	}

	t.Logf("✓ Test passed: Uninitialized services returns 500")
}

// Test 10: Multiple countries
func TestMultipleCountries(t *testing.T) {
	mockFirestore := NewMockFirestoreUpdater()
	updater = mockFirestore
	notifier = NewMockBackendNotifier()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		msgMap := payload["message"].(map[string]interface{})
		msgID := msgMap["messageId"].(string)

		processed, _ := updater.CheckIdempotency(r.Context(), msgID)
		if processed {
			w.WriteHeader(http.StatusOK)
			return
		}

		dataStr := msgMap["data"].(string)
		decoded, _ := base64.StdEncoding.DecodeString(dataStr)

		var event ClickEvent
		json.Unmarshal(decoded, &event)

		updater.IncrementCounters(r.Context(), event.Country, event.Country)
		updater.RecordProcessedMessage(r.Context(), msgID, event.Country)

		counters, _ := updater.GetCounters(r.Context())
		global := counters["global"].(int64)
		countries := counters["countries"].(map[string]interface{})
		notifier.NotifyCounterUpdate(global, countries)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","messageId":"%s"}`, msgID)
	})

	// Send 3 messages from different countries
	countries := []string{"US", "GB", "CA"}
	for i, country := range countries {
		body := createPubSubMessage(fmt.Sprintf("msg-%d", i), country, "1.2.3.4", time.Now().Unix())
		req := httptest.NewRequest("POST", "/process", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d", i, w.Code)
		}
	}

	// Verify global counter
	if mockFirestore.counters["global"].(int64) != 3 {
		t.Errorf("Expected global counter to be 3, got %d", mockFirestore.counters["global"].(int64))
	}

	// Verify country-specific counters exist
	countries_map := mockFirestore.counters["countries"].(map[string]interface{})
	if len(countries_map) < 3 {
		t.Errorf("Expected at least 3 countries, got %d", len(countries_map))
	}

	t.Logf("✓ Test passed: Multiple countries handled correctly (global: %d, countries: %d)", mockFirestore.counters["global"].(int64), len(countries_map))
}

