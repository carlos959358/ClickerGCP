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

	"cloud.google.com/go/firestore"
)

var (
	updater  *FirestoreUpdater
	notifier *BackendNotifier
)

// Helper to get map keys for debugging
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Save failed message to dead letter queue in Firestore
func saveFailedMessage(messageID string, payload map[string]interface{}, errorMsg string) error {
	if updater == nil {
		return fmt.Errorf("updater not initialized")
	}

	// Save to Firestore failed_messages collection
	ctx := context.Background()
	failedMsg := map[string]interface{}{
		"messageId":   messageID,
		"payload":     payload,
		"error":       errorMsg,
		"timestamp":   time.Now().UTC(),
		"failedCount": 1,
	}

	// Try to increment failure count if message already exists
	doc := updater.client.Collection("failed_messages").Doc(messageID)
	err := updater.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(doc)
		if err == nil && snap.Exists() {
			// Message already exists, increment count
			failedCount := int64(1)
			if val, ok := snap.Data()["failedCount"].(int64); ok {
				failedCount = val + 1
			}
			return tx.Set(doc, map[string]interface{}{
				"failedCount": failedCount,
				"lastError":   errorMsg,
				"lastAttempt": time.Now().UTC(),
			}, firestore.MergeAll)
		}
		// New message, create it
		return tx.Set(doc, failedMsg)
	})

	return err
}

func initializeServices(ctx context.Context, projectID, backendURL string) {
	log.Println("[Services] Initializing Firestore...")
	var err error
	updater, err = NewFirestoreUpdater(ctx, projectID)
	if err != nil {
		log.Printf("[Services] ✗ Firestore initialization failed: %v", err)
		return
	}
	log.Println("[Services] ✓ Firestore ready")

	log.Println("[Services] Initializing backend notifier...")
	notifier = NewBackendNotifier(backendURL)
	log.Println("[Services] ✓ Backend notifier ready")
}

func main() {
	// Configuration from environment
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		log.Fatal("GCP_PROJECT_ID environment variable not set")
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
	log.Printf("Project: %s, Backend: %s", projectID, backendURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize services (Firestore and backend notifier)
	go initializeServices(ctx, projectID, backendURL)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		status := "ready"
		if updater == nil || notifier == nil {
			status = "initializing"
		}
		fmt.Fprintf(w, `{"status":"%s","timestamp":%d}`, status, time.Now().UTC().Unix())
	})

	// Liveness probe endpoint
	http.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	// Pub/Sub push endpoint
	http.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		log.Printf("[/process] ===== START =====")

		// Accept any JSON structure
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("[/process] ERROR: JSON decode failed: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Raw payload decoded: %v", payload)

		// Extract message
		msgInterface, ok := payload["message"]
		if !ok {
			log.Printf("[/process] ERROR: No 'message' field in payload. Keys: %v", mapKeys(payload))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Message field found: %v", msgInterface)

		msgMap, ok := msgInterface.(map[string]interface{})
		if !ok {
			log.Printf("[/process] ERROR: Message is not a map, type: %T", msgInterface)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Message is map with keys: %v", mapKeys(msgMap))

		// Extract data field
		dataStr, ok := msgMap["data"].(string)
		if !ok {
			log.Printf("[/process] ERROR: No 'data' field or not string, type: %T, keys: %v", msgMap["data"], mapKeys(msgMap))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Data field found, length: %d bytes", len(dataStr))

		// Decode base64 data
		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			log.Printf("[/process] ERROR: Base64 decode failed: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Base64 decoded, result: %s", string(decoded))

		// Parse click event
		var event ClickEvent
		if err := json.Unmarshal(decoded, &event); err != nil {
			log.Printf("[/process] ERROR: Event unmarshal failed: %v", err)
			log.Printf("[/process] ERROR: Trying to unmarshal: %s", string(decoded))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Event parsed: Country=%s, IP=%s, Timestamp=%d", event.Country, event.IP, event.Timestamp)

		// Update Firestore
		if updater == nil {
			log.Printf("[/process] ERROR: Updater not initialized")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Updater initialized")

		if err := updater.IncrementCounters(context.Background(), event.Country, event.Country); err != nil {
			log.Printf("[/process] ERROR: Failed to increment counters: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Counters incremented for country: %s", event.Country)

		// Get updated counters
		counters, err := updater.GetCounters(context.Background())
		if err != nil {
			log.Printf("[/process] ERROR: Failed to get counters: %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		log.Printf("[/process] ✓ Counters retrieved: %v", counters)

		// Notify backend
		if notifier != nil {
			global := int64(0)
			if val, ok := counters["global"].(int64); ok {
				global = val
			}
			countries := make(map[string]interface{})
			if val, ok := counters["countries"].(map[string]interface{}); ok {
				countries = val
			}

			log.Printf("[/process] Notifying backend: global=%d, countries=%d", global, len(countries))
			if err := notifier.NotifyCounterUpdate(global, countries); err != nil {
				log.Printf("[/process] ERROR: Notify failed: %v", err)
			} else {
				log.Printf("[/process] ✓ Backend notified successfully")
			}
		} else {
			log.Printf("[/process] WARNING: Notifier not initialized, skipping backend notification")
		}

		log.Printf("[/process] ===== SUCCESS =====")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start HTTP server
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
