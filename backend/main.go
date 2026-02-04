package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/websocket"
)

// ClientMessage represents a message from client to server
type ClientMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// ServerMessage represents a message from server to client
type ServerMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// Client represents a connected WebSocket client
type Client struct {
	conn          *websocket.Conn
	send          chan interface{}
	token         string // Authentication token for this client
	clientIP      string // Client IP address
	country       string // Country code from geolocation
	lastClickTime time.Time
	clickCount    int
	mu            sync.Mutex
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	clients    map[*Client]bool
	tokens     map[string]*Client // Map of auth tokens to clients
	broadcast  chan interface{}
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		tokens:     make(map[string]*Client),
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.token != "" {
				h.tokens[client.token] = client
			}
			h.mu.Unlock()
			log.Printf("Client registered. Total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				if client.token != "" {
					delete(h.tokens, client.token)
				}
			}
			h.mu.Unlock()
			log.Printf("Client unregistered. Total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message interface{}) {
	h.broadcast <- message
}

// ValidateToken checks if a token is valid and belongs to an active client
func (h *Hub) ValidateToken(token string) bool {
	if token == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.tokens[token]
	return exists
}

// GenerateToken creates a new random authentication token
func GenerateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// checkRateLimit checks if a client has exceeded the rate limit (10 clicks per second)
func (c *Client) checkRateLimit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if now.Sub(c.lastClickTime) >= time.Second {
		// Reset counter every second
		c.clickCount = 0
		c.lastClickTime = now
	}

	if c.clickCount >= 10 {
		return false // Rate limit exceeded
	}

	c.clickCount++
	return true
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for WebSocket connections (same-origin only in practice)
		return true
	},
}

// getCountryFromIP looks up the country code for an IP address
func getCountryFromIP(ip string) string {
	// Skip geolocation for localhost and internal IPs
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return "LOCAL"
	}

	// Try ipapi.co API first (country_code endpoint returns just the code)
	if countryCode := tryIPAPIco(ip); countryCode != "Unknown" {
		return countryCode
	}

	// Fallback to ip-api.com
	if countryCode := tryIPAPI(ip); countryCode != "Unknown" {
		return countryCode
	}

	return "Unknown"
}

// tryIPAPIco attempts to get country code from ipapi.co
func tryIPAPIco(ip string) string {
	url := fmt.Sprintf("https://ipapi.co/%s/country_code/", ip)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "Unknown"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Unknown"
	}

	// Read response body as plain text (just the country code)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Unknown"
	}

	countryCode := strings.TrimSpace(string(bodyBytes))
	if countryCode != "" && countryCode != "None" {
		return countryCode
	}
	return "Unknown"
}

// tryIPAPI attempts to get country code from ip-api.com
func tryIPAPI(ip string) string {
	url := fmt.Sprintf("http://ip-api.com/json/%s", ip)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "Unknown"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Unknown"
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "Unknown"
	}

	// Extract country code from response
	if countryCode, ok := result["countryCode"].(string); ok && countryCode != "" {
		return countryCode
	}
	return "Unknown"
}

// WebSocket message handlers

// handleClick processes a click message from the client
func handleClick(client *Client, hub *Hub, ctx context.Context) {
	// Check rate limit
	if !client.checkRateLimit() {
		serverMsg := ServerMessage{
			Type: "click_error",
			Data: map[string]interface{}{
				"error": "rate limit exceeded",
			},
		}
		select {
		case client.send <- serverMsg:
		default:
			// Send channel full, skip
		}
		return
	}

	// Publish to Pub/Sub if available
	if publisher != nil {
		err := publisher.PublishClickEvent(ctx, client.country, client.clientIP)
		if err != nil {
			log.Printf("Failed to publish click event: %v", err)
		}
	}

	// Send success response
	serverMsg := ServerMessage{
		Type: "click_success",
		Data: map[string]interface{}{
			"status": "ok",
		},
	}
	select {
	case client.send <- serverMsg:
	default:
		// Send channel full, skip
	}
}

// handleGetCount sends the current count data to the client
func handleGetCount(client *Client, ctx context.Context) {
	var counterData *CounterData

	// Try to get data from Firestore if available
	if firestoreClient != nil {
		data, err := firestoreClient.GetCounters(ctx)
		if err != nil {
			log.Printf("ERROR reading from Firestore: %v", err)
			serverMsg := ServerMessage{
				Type: "count_error",
				Data: map[string]interface{}{
					"error": fmt.Sprintf("firestore error: %v", err),
				},
			}
			select {
			case client.send <- serverMsg:
			default:
			}
			return
		}
		counterData = data
	} else {
		// Fallback if Firestore not initialized
		counterData = &CounterData{
			Global: 0,
			Countries: map[string]interface{}{
				"country_US": map[string]interface{}{"count": int64(0), "country": "US"},
				"country_UK": map[string]interface{}{"count": int64(0), "country": "UK"},
				"country_DE": map[string]interface{}{"count": int64(0), "country": "DE"},
			},
		}
	}

	// Send count response
	serverMsg := ServerMessage{
		Type: "count_response",
		Data: map[string]interface{}{
			"global":    counterData.Global,
			"countries": counterData.Countries,
		},
	}
	select {
	case client.send <- serverMsg:
	default:
	}
}

// handleGetCountries sends the countries list to the client
func handleGetCountries(client *Client, ctx context.Context) {
	// Use default countries
	countries := map[string]interface{}{
		"country_US": map[string]interface{}{"count": int64(0), "country": "US"},
		"country_UK": map[string]interface{}{"count": int64(0), "country": "UK"},
		"country_DE": map[string]interface{}{"count": int64(0), "country": "DE"},
		"country_FR": map[string]interface{}{"count": int64(0), "country": "FR"},
		"country_JP": map[string]interface{}{"count": int64(0), "country": "JP"},
	}

	// Try to get real data from Firestore if available
	if firestoreClient != nil {
		if data, err := firestoreClient.GetCounters(ctx); err == nil {
			countries = data.Countries
		}
	}

	// Send countries response
	serverMsg := ServerMessage{
		Type: "countries_response",
		Data: map[string]interface{}{
			"countries": countries,
		},
	}
	select {
	case client.send <- serverMsg:
	default:
	}
}

// Global variables for debugging
var (
	projectID        string
	firestoreClient  *FirestoreClient
	publisher        *PubSubPublisher
	publisherError   string
)

// PubSubPublisher handles publishing messages to Pub/Sub
type PubSubPublisher struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// NewPubSubPublisher creates a new publisher
func NewPubSubPublisher(ctx context.Context, projectID, topicName string) (*PubSubPublisher, error) {
	log.Printf("[PubSubPublisher] Creating Pub/Sub client for project: %s", projectID)

	// Create a context with timeout for the initialization
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := pubsub.NewClient(initCtx, projectID)
	if err != nil {
		log.Printf("[PubSubPublisher] Failed to create client: %v", err)
		return nil, err
	}
	log.Printf("[PubSubPublisher] Client created successfully")

	// Get topic reference (we assume the topic exists, as checking existence
	// can fail with PermissionDenied in Cloud Run even with proper IAM roles)
	log.Printf("[PubSubPublisher] Getting topic reference for '%s'", topicName)
	topic := client.Topic(topicName)
	log.Printf("[PubSubPublisher] Topic reference obtained, assuming topic exists")

	log.Printf("[PubSubPublisher] Publisher ready for topic '%s'", topicName)
	return &PubSubPublisher{
		client: client,
		topic:  topic,
	}, nil
}

// PublishClickEvent publishes a click event to Pub/Sub
func (p *PubSubPublisher) PublishClickEvent(ctx context.Context, country, ip string) error {
	event := map[string]interface{}{
		"timestamp": time.Now().UTC().Unix(),
		"country":   country,
		"ip":        ip,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	result := p.topic.Publish(ctx, &pubsub.Message{Data: data})
	_, err = result.Get(ctx)
	return err
}

// Close closes the publisher
func (p *PubSubPublisher) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	projectID = os.Getenv("GCP_PROJECT_ID")

	// Create a background context that lives for the lifetime of the server
	bgCtx := context.Background()

	// Create and start the WebSocket hub
	hub := NewHub()
	go hub.Run()

	// Initialize Firestore client for reading/writing counter data
	if projectID != "" {
		log.Printf("Initializing Firestore for project: %s", projectID)
		var err error
		firestoreClient, err = NewFirestoreClient(bgCtx, projectID)
		if err != nil {
			log.Printf("ERROR: Failed to initialize Firestore: %v", err)
			log.Println("Continuing without Firestore integration...")
			firestoreClient = nil
		} else {
			defer firestoreClient.Close()
			log.Println("✓ Firestore client initialized successfully")
		}
	} else {
		log.Println("WARNING: GCP_PROJECT_ID not set, Firestore disabled")
	}

	// Initialize Pub/Sub publisher (optional - may not be needed in all environments)
	if projectID != "" {
		var err error
		publisher, err = NewPubSubPublisher(bgCtx, projectID, "click-events")
		if err != nil {
			log.Printf("ERROR: Failed to initialize Pub/Sub publisher: %v", err)
			publisherError = fmt.Sprintf("%v", err)
			log.Println("Continuing without Pub/Sub publishing...")
			publisher = nil
		} else {
			defer publisher.Close()
			log.Printf("✓ Pub/Sub publisher initialized for topic 'click-events'")
		}
	} else {
		log.Println("WARNING: GCP_PROJECT_ID not set, Pub/Sub disabled")
	}

	// API handlers
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})


	// WebSocket handler
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		// Generate authentication token for this client
		token := GenerateToken()

		// Extract client IP
		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
			clientIP = strings.TrimSpace(clientIP)
		} else {
			if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
				clientIP = clientIP[:idx]
			}
		}

		// Determine country from IP
		country := getCountryFromIP(clientIP)

		client := &Client{
			conn:          conn,
			send:          make(chan interface{}, 256),
			token:         token,
			clientIP:      clientIP,
			country:       country,
			lastClickTime: time.Now(),
		}
		hub.register <- client

		// Send the token to the client immediately
		if err := conn.WriteJSON(map[string]interface{}{
			"type":  "auth_token",
			"token": token,
		}); err != nil {
			log.Printf("Failed to send auth token: %v", err)
			conn.Close()
			return
		}
		log.Printf("Sent auth token to client: %s from %s (%s)", token[:8]+"...", clientIP, country)

		// Request initial counter data via message handler
		go func() {
			// Small delay to ensure client is ready
			time.Sleep(100 * time.Millisecond)
			handleGetCount(client, bgCtx)
		}()

		go func() {
			defer func() {
				hub.unregister <- client
				conn.Close()
			}()

			// Read messages from client
			for {
				var clientMsg ClientMessage
				if err := conn.ReadJSON(&clientMsg); err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						log.Printf("WebSocket error: %v", err)
					}
					return
				}

				// Handle different message types
				switch clientMsg.Type {
				case "click":
					handleClick(client, hub, bgCtx)

				case "get_count":
					handleGetCount(client, bgCtx)

				case "get_countries":
					handleGetCountries(client, bgCtx)

				default:
					log.Printf("Unknown message type: %s", clientMsg.Type)
				}
			}
		}()

		// Write messages to client
		for {
			message, ok := <-client.send
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteJSON(message); err != nil {
				log.Printf("Write error: %v", err)
				return
			}
		}
	})

	// Config endpoint - shows initialization status
	mux.HandleFunc("/debug/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		pubErrorStr := "null"
		if publisherError != "" {
			pubErrorStr = fmt.Sprintf("\"%s\"", publisherError)
		}
		fmt.Fprintf(w, `{
  "projectID": "%s",
  "firestoreClient": %v,
  "pubsubPublisher": %v,
  "publisherError": %s
}`, projectID, firestoreClient != nil, publisher != nil, pubErrorStr)
	})

	// Debug endpoint - shows all Firestore documents
	mux.HandleFunc("/debug/firestore", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if firestoreClient == nil {
			w.Write([]byte(`{"error":"firestore not initialized"}`))
			return
		}

		data, err := firestoreClient.GetCounters(bgCtx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"%v"}`, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"global":    data.Global,
			"countries": data.Countries,
		})
	})

	// Broadcast endpoint - used by consumer to send updates to all connected clients
	mux.HandleFunc("/internal/broadcast", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"error":"method not allowed"}`))
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid json"}`))
			return
		}

		// Broadcast to all WebSocket clients
		hub.Broadcast(payload)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		log.Printf("Broadcast sent to %d clients", len(hub.clients))
	})

	// Serve static files (frontend)
	staticDir := filepath.Join(".", "static")
	fs := http.FileServer(http.Dir(staticDir))

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		path := filepath.Join(staticDir, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// If file doesn't exist and it's not an API call, serve index.html (SPA routing)
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}))

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
