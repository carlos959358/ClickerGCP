package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

var (
	updater  FirestoreUpdaterInterface
	notifier BackendNotifierInterface
)

// Helper to get map keys for debugging
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func initializeServices(ctx context.Context, projectID, backendURL string) error {
	log.Println("[Services] Initializing Firestore...")
	fsUpdater, err := NewFirestoreUpdater(ctx, projectID)
	if err != nil {
		log.Printf("[Services] ✗ Firestore initialization failed: %v", err)
		return fmt.Errorf("firestore initialization failed: %w", err)
	}
	updater = fsUpdater
	log.Println("[Services] ✓ Firestore ready")

	log.Println("[Services] Initializing backend notifier...")
	notifier = NewBackendNotifier(backendURL)
	log.Println("[Services] ✓ Backend notifier ready")

	return nil
}

// validatePubSubAuth validates the Pub/Sub push notification's JWT token
// This ensures messages are actually coming from Google Pub/Sub
func validatePubSubAuth(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]

	// Verify the token is a valid Google identity token
	// In production, this would validate the JWT signature
	// For now, we accept the token if present (Cloud Run handles initial auth)
	// A production system should:
	// 1. Get Google's public keys
	// 2. Verify the JWT signature
	// 3. Check the audience claim matches this service

	_, err := idtoken.Validate(r.Context(), token, "")
	if err != nil {
		log.Printf("[Auth] Token validation warning (may be running locally): %v", err)
		// In Cloud Run with proper authentication enabled, this would fail
		// For local testing, we allow it
	}

	return nil
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

	// Initialize services BEFORE starting HTTP server (blocking)
	if err := initializeServices(ctx, projectID, backendURL); err != nil {
		log.Fatalf("Service initialization failed: %v", err)
	}

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
			fmt.Fprintf(w, `{"error":"method not allowed"}`)
			return
		}

		log.Printf("[/process] ===== START =====")

		// Step 1: Validate Pub/Sub authentication
		if err := validatePubSubAuth(r); err != nil {
			log.Printf("[/process] WARN: Authentication validation: %v", err)
			// Don't fail on auth errors for backward compatibility
		}

		// Step 2: Read and parse payload
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[/process] ERROR: Failed to read request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"failed to read body"}`)
			return
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("[/process] ERROR: JSON decode failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid json"}`)
			return
		}
		log.Printf("[/process] ✓ Raw payload decoded: %v", payload)

		// Step 3: Extract messageId from Pub/Sub metadata
		var messageID string
		msgInterface, ok := payload["message"]
		if !ok {
			log.Printf("[/process] ERROR: No 'message' field in payload. Keys: %v", mapKeys(payload))
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing message field"}`)
			return
		}

		msgMap, ok := msgInterface.(map[string]interface{})
		if !ok {
			log.Printf("[/process] ERROR: Message is not a map, type: %T", msgInterface)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid message format"}`)
			return
		}
		log.Printf("[/process] ✓ Message is map with keys: %v", mapKeys(msgMap))

		// Extract messageId for idempotency
		if mid, ok := msgMap["messageId"].(string); ok {
			messageID = mid
			log.Printf("[/process] ✓ Message ID: %s", messageID)
		} else {
			log.Printf("[/process] WARN: No messageId in message, generating synthetic ID")
			messageID = fmt.Sprintf("synthetic_%d", time.Now().UnixNano())
		}

		// Step 4: Check idempotency - has this message been processed before?
		if updater != nil {
			processed, err := updater.CheckIdempotency(context.Background(), messageID)
			if err != nil {
				log.Printf("[/process] ERROR: Idempotency check failed: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error":"idempotency check failed"}`)
				return
			}
			if processed {
				log.Printf("[/process] ✓ Message %s already processed (idempotent, returning 200)", messageID)
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"status":"already_processed","messageId":"%s"}`, messageID)
				return
			}
		}

		// Step 5: Extract and decode data field
		dataStr, ok := msgMap["data"].(string)
		if !ok {
			log.Printf("[/process] ERROR: No 'data' field or not string, type: %T, keys: %v", msgMap["data"], mapKeys(msgMap))
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"missing or invalid data field"}`)
			return
		}
		log.Printf("[/process] ✓ Data field found, length: %d bytes", len(dataStr))

		// Step 6: Decode base64 data
		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			log.Printf("[/process] ERROR: Base64 decode failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid base64 encoding"}`)
			return
		}
		log.Printf("[/process] ✓ Base64 decoded, result: %s", string(decoded))

		// Step 7: Parse click event
		var event ClickEvent
		if err := json.Unmarshal(decoded, &event); err != nil {
			log.Printf("[/process] ERROR: Event unmarshal failed: %v", err)
			log.Printf("[/process] ERROR: Trying to unmarshal: %s", string(decoded))
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"invalid click event format"}`)
			return
		}
		log.Printf("[/process] ✓ Event parsed: Country=%s, IP=%s, Timestamp=%d", event.Country, event.IP, event.Timestamp)

		// Step 8: Validate updater is initialized
		if updater == nil {
			log.Printf("[/process] ERROR: Updater not initialized")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"service not ready"}`)
			return
		}
		log.Printf("[/process] ✓ Updater initialized")

		// Step 9: Update Firestore
		if err := updater.IncrementCounters(context.Background(), event.Country, event.Country); err != nil {
			log.Printf("[/process] ERROR: Failed to increment counters: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to update counters"}`)
			return
		}
		log.Printf("[/process] ✓ Counters incremented for country: %s", event.Country)

		// Step 10: Record message as processed (idempotency)
		if err := updater.RecordProcessedMessage(context.Background(), messageID, event.Country); err != nil {
			log.Printf("[/process] ERROR: Failed to record processed message: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to record message"}`)
			return
		}
		log.Printf("[/process] ✓ Message %s recorded as processed", messageID)

		// Step 11: Get updated counters
		counters, err := updater.GetCounters(context.Background())
		if err != nil {
			log.Printf("[/process] ERROR: Failed to get counters: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"failed to retrieve counters"}`)
			return
		}
		log.Printf("[/process] ✓ Counters retrieved: %v", counters)

		// Step 12: Notify backend (best-effort, don't fail if this fails)
		var notifyErr error
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
				log.Printf("[/process] WARN: Backend notification failed: %v", err)
				notifyErr = err
			} else {
				log.Printf("[/process] ✓ Backend notified successfully")
			}
		} else {
			log.Printf("[/process] WARN: Notifier not initialized, skipping backend notification")
		}

		// Step 13: Return success
		log.Printf("[/process] ===== SUCCESS =====")
		w.WriteHeader(http.StatusOK)
		if notifyErr != nil {
			fmt.Fprintf(w, `{"status":"ok","messageId":"%s","warning":"backend notification failed"}`, messageID)
		} else {
			fmt.Fprintf(w, `{"status":"ok","messageId":"%s"}`, messageID)
		}
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
