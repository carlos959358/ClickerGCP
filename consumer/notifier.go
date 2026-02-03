package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type BackendNotifier struct {
	backendURL string
	client     *http.Client
}

func NewBackendNotifier(backendURL string) *BackendNotifier {
	log.Printf("[Notifier] Initializing BackendNotifier with URL: %s", backendURL)
	return &BackendNotifier{
		backendURL: backendURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type BroadcastPayload struct {
	Type      string                 `json:"type"`
	Global    int64                  `json:"global"`
	Countries map[string]interface{} `json:"countries"`
}

func (b *BackendNotifier) NotifyCounterUpdate(global int64, countries map[string]interface{}) error {
	log.Printf("[Notifier] NotifyCounterUpdate: global=%d, countries=%d", global, len(countries))

	payload := BroadcastPayload{
		Type:      "counter_update",
		Global:    global,
		Countries: countries,
	}

	log.Printf("[Notifier] Marshaling payload to JSON")
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[Notifier] ERROR: Failed to marshal payload: %v", err)
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	log.Printf("[Notifier] ✓ Payload marshaled, size: %d bytes", len(data))

	url := fmt.Sprintf("%s/internal/broadcast", b.backendURL)
	log.Printf("[Notifier] POSTing to URL: %s", url)

	resp, err := b.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("[Notifier] ERROR: Failed to POST to backend: %v", err)
		return fmt.Errorf("failed to notify backend: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[Notifier] ✓ Response received with status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[Notifier] ERROR: Failed to read response body: %v", err)
			return fmt.Errorf("backend returned status %d, failed to read body: %w", resp.StatusCode, err)
		}
		respBody := string(body)
		log.Printf("[Notifier] ERROR: Backend returned non-OK status %d with body: %s", resp.StatusCode, respBody)
		return fmt.Errorf("backend returned status %d: %s", resp.StatusCode, respBody)
	}

	log.Printf("[Notifier] ✓ Backend notification successful")
	return nil
}
