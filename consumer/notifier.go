package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BackendNotifier struct {
	backendURL string
	client     *http.Client
}

func NewBackendNotifier(backendURL string) *BackendNotifier {
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
	payload := BroadcastPayload{
		Type:      "counter_update",
		Global:    global,
		Countries: countries,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/internal/broadcast", b.backendURL)
	resp, err := b.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to notify backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
