# Clicker GCP - Architecture Documentation

## Overview

The Clicker application is a real-time distributed counter game deployed on Google Cloud Platform. Users click buttons to increment global and country-specific counters, with real-time synchronization via WebSocket.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (Browser)                       │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • Click Button → HTTP POST /click                         │  │
│  │ • WebSocket Connection → Receive Real-time Updates       │  │
│  │ • Display Global & Country Leaderboards                  │  │
│  └───────────────────────────────────────────────────────────┘  │
└──────────────────┬──────────────────────────────────────────────┘
                   │ HTTP + WebSocket
┌──────────────────▼──────────────────────────────────────────────┐
│              Backend Service (Cloud Run - Go)                    │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ • Handlers:                                              │  │
│  │   - POST /click → Geolocate IP → Publish to Pub/Sub    │  │
│  │   - GET /count → Read from Firestore                   │  │
│  │   - GET /countries → List country counters             │  │
│  │   - GET /health → Health check                         │  │
│  │   - WebSocket /ws → Manage client connections          │  │
│  │   - POST /internal/broadcast → Receive from Consumer   │  │
│  │                                                         │  │
│  │ • Services:                                             │  │
│  │   - Geolocation: IP → Country (with caching)          │  │
│  │   - Pub/Sub: Publish click events                      │  │
│  │   - Firestore: Read-only access to counters           │  │
│  │   - WebSocket Manager: Broadcast to all clients        │  │
│  └───────────────────────────────────────────────────────────┘  │
└──────────┬──────────────────────────────────────────┬───────────┘
           │ Pub/Sub Publish                          │ WebSocket
           │ (click events)                           │ (broadcasts)
           ▼                                          │
┌──────────────────────────────────────────────────┐  │
│         Google Cloud Pub/Sub (Topic)             │  │
│  ┌────────────────────────────────────────────┐  │  │
│  │ • click-events (Topic)                     │  │  │
│  │ • click-consumer-sub (Subscription)       │  │  │
│  │ • 600s message retention                  │  │  │
│  │ • At-least-once delivery guarantee        │  │  │
│  └────────────────────────────────────────────┘  │  │
└──────────────────┬───────────────────────────────┘  │
                   │ Pub/Sub Push                     │
┌──────────────────▼──────────────────────────────────▼──────────┐
│           Consumer Service (Cloud Run - Go)                    │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │ • Receives messages from Pub/Sub                       │  │
│  │ • Parses click events (timestamp, country, IP)        │  │
│  │ • Updates Firestore counters (atomic transactions)    │  │
│  │ • Notifies backend via /internal/broadcast            │  │
│  │ • Health check endpoint: /health, /live               │  │
│  │ • Concurrent message processing (configurable workers)│  │
│  └─────────────────────────────────────────────────────────┘  │
└──────────────────┬──────────────────────────────────────────────┘
                   │ Firestore Write
┌──────────────────▼──────────────────────────────────────────────┐
│                    Google Cloud Firestore                       │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │ Collection: counters/                                  │  │
│  │  ├── global (Document)                                │  │
│  │  │   ├── count: Integer                              │  │
│  │  │   └── lastUpdated: Timestamp                      │  │
│  │  │                                                    │  │
│  │  ├── country_US (Document)                           │  │
│  │  │   ├── country: "United States"                   │  │
│  │  │   ├── count: Integer                             │  │
│  │  │   └── lastUpdated: Timestamp                     │  │
│  │  │                                                   │  │
│  │  └── country_* (Multiple documents, one per country)│  │
│  │      └── ...                                        │  │
│  └─────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## Data Flow

### 1. User Click Event

```
1. User clicks button in browser
   ↓
2. Frontend sends: POST /click
   ↓
3. Backend receives request
   ├─ Extract client IP from request headers
   ├─ Geolocate IP to country (with 1-hour cache)
   └─ Publish click event to Pub/Sub: {timestamp, country, ip}
   ↓
4. Return HTTP 200 OK to frontend
```

### 2. Counter Update Processing

```
1. Consumer receives message from Pub/Sub
   ↓
2. Parse click event (country, timestamp, IP)
   ↓
3. Start Firestore transaction (atomic)
   ├─ Increment counters/global.count += 1
   ├─ Increment/Create counters/country_{CODE}.count += 1
   └─ Commit transaction
   ↓
4. Fetch updated counter values
   ├─ Global count
   └─ All country counts
   ↓
5. Send HTTP POST to backend: /internal/broadcast
   └─ Payload: {type: "counter_update", global: N, countries: {...}}
   ↓
6. Backend receives broadcast
   ├─ Queue message to all WebSocket clients
   └─ Broadcast to all connected users
   ↓
7. Frontend receives WebSocket message
   ├─ Parse counter_update
   ├─ Update displayed counts
   └─ Update leaderboard
   ↓
8. User sees counters updated in real-time
```

## Component Details

### Frontend (Static Web Application)

**Technology**: HTML5, CSS3, JavaScript (ES6+)

**Features**:
- Click button to submit clicks
- Real-time WebSocket connection for counter updates
- Display global counter and country leaderboard
- Connection status indicator
- Responsive design (mobile-friendly)
- Flag emojis for country display

**Key Files**:
- `frontend/index.html` - Main page structure
- `frontend/js/app.js` - WebSocket client and event handling
- `frontend/css/style.css` - Responsive styling

**API Usage**:
- `POST /click` - Send a click
- `GET /count` - Fetch current counters
- `GET /countries` - Fetch country leaderboard
- `WebSocket /ws` - Real-time updates

### Backend Service

**Technology**: Go 1.22 with gorilla/websocket

**Key Responsibilities**:
1. Accept click requests from frontend
2. Geolocate IP addresses to countries
3. Publish click events to Pub/Sub
4. Manage WebSocket connections to frontend
5. Broadcast counter updates to all clients
6. Provide health checks for Cloud Run

**Key Components**:

#### Handlers (handlers/)
- `api.go` - HTTP endpoints for click, count, countries, health, broadcast
- `websocket.go` - WebSocket connection manager and broadcaster

#### Services (services/)
- `geolocation.go` - IP geolocation with caching
- `pubsub.go` - Pub/Sub client and message publishing
- `firestore.go` - Firestore read operations

#### Models (models/)
- `types.go` - Data structures (ClickEvent, CounterUpdate, etc.)

**Configuration**:
- `GCP_PROJECT_ID` - GCP project ID
- `PUBSUB_TOPIC` - Pub/Sub topic name (default: click-events)
- `FIRESTORE_DATABASE` - Firestore database ID (default: clicker-db)
- `PORT` - HTTP listen port (default: 8080)

**Resource Limits**:
- Memory: 512 MB
- CPU: 1 CPU
- Max concurrent connections: ~1000 WebSocket connections
- Timeout: 300 seconds

### Consumer Service

**Technology**: Go 1.22 with cloud.google.com/go/pubsub and cloud.google.com/go/firestore

**Key Responsibilities**:
1. Subscribe to Pub/Sub messages
2. Parse and validate click events
3. Update Firestore counters atomically
4. Notify backend of updates for broadcasting
5. Provide health metrics

**Key Components**:
- `main.go` - Entry point, Pub/Sub subscription setup
- `subscriber.go` - Message handler and processor
- `firestore.go` - Atomic counter updates
- `notifier.go` - Backend notification client

**Configuration**:
- `GCP_PROJECT_ID` - GCP project ID
- `PUBSUB_SUBSCRIPTION` - Pub/Sub subscription name
- `FIRESTORE_DATABASE` - Firestore database ID
- `BACKEND_URL` - Backend service URL for notifications
- `PORT` - Health check server port (default: 8080)

**Resource Limits**:
- Memory: 512 MB
- CPU: 1 CPU
- Max concurrent message processing: 10 workers
- Ack deadline: 60 seconds

### Firestore Database

**Schema**:
```
counters/
├── global
│   ├── count: Number (Increment value)
│   └── lastUpdated: Timestamp
└── country_{CODE}  (e.g., country_US, country_JP)
    ├── country: String (Full country name)
    ├── count: Number (Increment value)
    └── lastUpdated: Timestamp
```

**Operations**:
- **Write** (Consumer Service):
  - Atomic transactions via Firestore SDK
  - `Increment(1)` for atomicity
  - Creates country documents on first click

- **Read** (Backend Service):
  - Batch read all documents in collection
  - No transactions needed (eventually consistent)
  - Cached implicitly by clients

**Scalability**:
- Single `global` document can receive ~1 write/second
- Bottleneck: Firestore write rate limits per document
- Mitigation (future): Distributed counter pattern (shard across multiple documents)

### Pub/Sub Topic

**Configuration**:
- Topic: `click-events`
- Subscription: `click-consumer-sub`
- Message retention: 600 seconds (10 minutes)
- Delivery guarantee: At-least-once
- Push delivery to consumer service

**Message Format**:
```json
{
  "timestamp": "2025-01-01T12:00:00Z",
  "country": "United States",
  "ip": "203.0.113.42"
}
```

**Backpressure**:
- Pub/Sub buffering handles traffic spikes
- Consumer processes messages asynchronously
- Failed messages automatically retried

## Data Consistency

### Eventual Consistency Model

The system uses eventual consistency:

1. **Immediate**: Click is received and published to Pub/Sub
2. **Near-immediate** (100-500ms): Message processed, Firestore updated
3. **Final** (< 1s): WebSocket broadcast received, frontend updated

All users eventually see the same counter values, but there may be temporary divergence between different clients due to network latency and processing time.

### Atomicity

Firestore transactions ensure:
- Global counter and country counter increments are atomic
- Either both increment or both fail (no partial updates)
- Consumer retries message on failure

## Geolocation Strategy

### Primary: ipapi.co
- Free tier: 30,000 requests/month
- Response time: ~200ms
- Accuracy: Country level

### Fallback: ip-api.com
- Used if primary fails
- Response time: ~150ms
- Accuracy: Country level

### Caching
- 1-hour TTL per IP address
- In-memory cache on backend service
- Reduces external API calls and improves latency
- Periodic cleanup of expired entries

## Scalability

### Current Limits

| Metric | Limit | Notes |
|--------|-------|-------|
| Concurrent WebSocket Connections | ~1000 | Per backend instance |
| Geolocation Cache Size | ~100K IPs | In-memory, 1GB+ for full cache |
| Firestore Writes | 1/sec per document | Global counter bottleneck |
| Pub/Sub Messages | 10K/sec per topic | Well above expected traffic |

### Scaling Strategy

**Horizontal Scaling**:
- Backend: Cloud Run auto-scales (0-100 instances)
- Consumer: Cloud Run auto-scales (0-50 instances)
- Both scale based on request/message rate

**Vertical Scaling** (if needed):
- Increase CPU and memory in terraform variables
- Adjust Cloud Run settings

**Bottleneck Mitigation**:
- **Firestore write limits**: Implement distributed counter pattern
  - Shard global counter: `global_0`, `global_1`, etc.
  - Read all shards and sum for final count
  - Enables 10x more writes

- **WebSocket connections**:
  - Multi-region deployment with load balancing
  - Use Cloud Pub/Sub for cross-region communication

- **Geolocation API limits**:
  - Implement IP geolocation caching layer
  - Use Redis/Memorystore for distributed cache

## Security Considerations

### Authentication & Authorization

- **Frontend**: No auth required (public game)
- **Backend**:
  - `/click` → Public (rate limiting recommended)
  - `/internal/broadcast` → Internal only (pod-to-pod communication)
  - `/ws` → Public via HTTPS/WSS only

- **Consumer**:
  - Receives messages via Pub/Sub (service account auth)
  - Calls backend with service account context

### Network Security

- **Cloud Run**: Only HTTPS/WSS (TLS 1.2+)
- **Service-to-Service**: Service account identity verification
- **Pub/Sub**: Message encryption in transit and at rest
- **Firestore**: Service account IAM permissions

### Recommended Enhancements

- **Cloud Armor**: Add DDoS protection rules
- **Cloud VPC**: Isolate backend/consumer from internet (optional)
- **Rate Limiting**: Implement per-IP click rate limiting
- **Input Validation**: Validate geolocation responses
- **Audit Logging**: Enable Cloud Audit Logs

## Monitoring & Observability

### Cloud Run Metrics

Automatically collected:
- Request count and latency
- Error rates
- Memory and CPU usage
- Container startup time

### Custom Metrics

Available via Cloud Logging:
- Message processing rates (consumer)
- Firestore operation latencies
- WebSocket connection count
- Geolocation cache hit rates

### Logging

All services log to Cloud Logging:
- Backend: Request logs, WebSocket events, geolocations
- Consumer: Message processing, Firestore operations, errors
- Firestore: Document operations (optional)

### Health Checks

- **Backend**: `GET /health` → Returns JSON status
- **Consumer**: `GET /health` → Returns processing stats
- Cloud Run: HTTP liveness probes (configurable)

## Cost Optimization

### Current Estimate (Low Traffic)

| Service | Usage | Cost |
|---------|-------|------|
| Cloud Run (Backend) | 0.1 CPU-hours/day | $5-10/month |
| Cloud Run (Consumer) | 0.05 CPU-hours/day | $2-5/month |
| Pub/Sub | <1GB/month | Free |
| Firestore | <1M reads/month | Free |
| Geolocation Cache | ~100K IPs | Free (local) |
| **Total** | | **$10-15/month** |

### Cost Reduction Strategies

1. **Set min-instances to 0** (default)
   - Pay only for actual usage
   - Accept 1-3 second cold start time

2. **Implement aggressive caching**
   - 1-hour geolocation cache
   - Browser-side caching for static assets

3. **Monitor and optimize**
   - Set up cost alerts
   - Monitor Firestore operations
   - Adjust instance scaling

4. **Compression**
   - Enable gzip on backend
   - Minify frontend assets

## Deployment Architecture

### Terraform IaC

All infrastructure defined in code:
- Firestore database
- Pub/Sub topic and subscription
- Cloud Run services
- Service accounts and IAM
- API enablement

### CI/CD Pipeline (Optional)

Recommended setup:
1. Push code to GitHub
2. Cloud Build triggers:
   - Build Docker images
   - Push to Artifact Registry
   - Update Cloud Run services
   - Run tests

### Disaster Recovery

- **Data**: Firestore automatic backups
- **Service**: Cloud Run auto-restarts failed instances
- **Code**: Version control (GitHub)
- **Configuration**: Terraform state (versioned)

## Future Enhancements

1. **Real-time Firestore Listeners**
   - Remove Pub/Sub consumer notification latency
   - Backend listens to Firestore changes directly
   - Broadcast immediately on data change

2. **Distributed Counters**
   - Shard global counter for 10x throughput
   - Aggregate across shards in read path

3. **Redis Caching**
   - Memorystore for cross-region geolocation cache
   - Reduce API calls to external services

4. **Multi-region Deployment**
   - Deploy to multiple regions for latency
   - Firestore replication for local reads

5. **Analytics**
   - BigQuery export for click analytics
   - Dashboards for trends and patterns

6. **Advanced Features**
   - User accounts and leaderboards
   - Achievements and badges
   - Power-ups and special events
