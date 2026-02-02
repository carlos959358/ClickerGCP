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
)

var (
	updater  *FirestoreUpdater
	notifier *BackendNotifier
)

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

		// Parse Pub/Sub message
		var pubsubMessage struct {
			Message struct {
				Data string `json:"data"`
				ID   string `json:"messageId"`
			} `json:"message"`
		}

		if err := json.NewDecoder(r.Body).Decode(&pubsubMessage); err != nil {
			log.Printf("Failed to decode message: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Decode base64 data
		decoded, err := base64.StdEncoding.DecodeString(pubsubMessage.Message.Data)
		if err != nil {
			log.Printf("Base64 decode failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse click event
		var event ClickEvent
		if err := json.Unmarshal(decoded, &event); err != nil {
			log.Printf("Event unmarshal failed: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Update Firestore
		if updater == nil {
			log.Printf("Updater not initialized")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := updater.IncrementCounters(context.Background(), event.Country, event.Country); err != nil {
			log.Printf("Failed to increment: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Notify backend
		counters, err := updater.GetCounters(context.Background())
		if err != nil {
			log.Printf("Failed to get counters: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if notifier != nil {
			global := int64(0)
			if val, ok := counters["global"].(int64); ok {
				global = val
			}
			countries := make(map[string]interface{})
			if val, ok := counters["countries"].(map[string]interface{}); ok {
				countries = val
			}

			if err := notifier.NotifyCounterUpdate(global, countries); err != nil {
				log.Printf("Notify failed: %v", err)
			}
		}

		// Success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
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
