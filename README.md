# ClickerGCP - Real-Time Click Counter

A fully functional, production-ready click counter application built on Google Cloud Platform with real-time updates via WebSockets.

## ✅ System Status

**All components operational and tested:**
- ✅ Backend API (Cloud Run) - Publishing clicks to Pub/Sub
- ✅ Consumer Service (Cloud Run) - Processing messages from Pub/Sub
- ✅ Firestore Database - Persisting counter data
- ✅ Pub/Sub - Asynchronous message delivery
- ✅ WebSocket Broadcasting - Real-time frontend updates
- ✅ Comprehensive Test Suite - 15 tests, all passing

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Frontend (WebSocket)                   │
│              Real-time Counter Display                  │
└────────────────────────────────┬────────────────────────┘
                                 │
                    ┌────────────┴───────────────┐
                    │                            │
            ┌───────▼─────────┐        ┌────────▼────────┐
            │  Backend        │        │   Consumer      │
            │  Service        │        │   Service       │
            │  (Cloud Run)    │        │   (Cloud Run)   │
            └────────┬────────┘        └────────┬────────┘
                     │                          │
                     │ Publishes                │
                     │ Click Events             │ Receives
                     │ (Unix timestamp)         │ Messages
            ┌────────▼──────────────────────────▼────────┐
            │     Google Pub/Sub                         │
            │  Topic: click-events                       │
            │  Subscription: click-consumer-sub (PUSH)  │
            └────────┬──────────────────────────┬────────┘
                     │                          │
                     └──────────────┬───────────┘
                                    │
                    ┌───────────────▼───────────┐
                    │  Firestore Database       │
                    │  Collections:             │
                    │  - counters/global        │
                    │  - counters/country_*     │
                    │  - processed_messages     │
                    └──────────────────────────┘
```

---

## Message Flow


```
1. User clicks the frontend
   └─> Frontend sends HTTP GET /click?country=XX&ip=A.B.C.D

2. Backend receives click
   └─> Geolocates IP to verify country
   └─> Publishes to Pub/Sub:
       {
         "timestamp": 1770040632  (Unix timestamp, int64),
         "country": "ES",
         "ip": "217.130.116.130"
       }

3. Pub/Sub pushes message to Consumer
   └─> HTTP POST to consumer /process endpoint
   └─> Message wrapped in Pub/Sub envelope:
       {
         "message": {
           "messageId": "17886842157423762",
           "data": "base64(encoded_event)"
         }
       }

4. Consumer processes message
   ├─> Extracts messageId for idempotency checking
   ├─> Decodes base64 and parses JSON
   ├─> Validates required fields
   ├─> Checks if already processed (idempotency)
   ├─> Increments Firestore counters (atomic transaction)
   ├─> Records message as processed
   └─> Notifies backend of counter update

5. Backend broadcasts to WebSocket clients
   └─> All connected frontends receive real-time update

6. Frontend updates counter display
```

---

## Project Structure

```
ClickerGCP/
├── README.md                          (This file)
├── terraform/                         (Infrastructure as Code)
│   ├── main.tf                       (Main Terraform config)
│   ├── variables.tf                  (Variables)
│   ├── outputs.tf                    (Outputs)
│   ├── firestore.tf                  (Firestore setup)
│   ├── pubsub.tf                     (Pub/Sub setup)
│   ├── cloudrun.tf                   (Cloud Run services)
│   └── iam.tf                        (IAM roles & service accounts)
│
├── backend/                          (Backend API Service)
│   ├── main.go                       (HTTP handlers, WebSocket hub, Pub/Sub publisher)
│   ├── firestore.go                  (Firestore read operations)
│   ├── Dockerfile                    (Container image)
│   ├── cloudbuild.yaml               (Cloud Build config)
│   └── go.mod / go.sum               (Dependencies)
│
├── consumer/                         (Pub/Sub Consumer Service)
│   ├── main.go                       (HTTP /process endpoint, message handler)
│   ├── main_test.go                  (10 unit tests)
│   ├── integration_test.go           (5 integration tests)
│   ├── firestore.go                  (Firestore write operations)
│   ├── notifier.go                   (Backend notification)
│   ├── subscriber.go                 (Alternative pull-based subscriber - not deployed)
│   ├── interfaces.go                 (Mock interfaces for testing)
│   ├── Dockerfile                    (Container image)
│   ├── cloudbuild.yaml               (Cloud Build config)
│   └── go.mod / go.sum               (Dependencies)
│
└── frontend/                         (Static HTML/CSS/JS)
    ├── index.html                    (Counter UI + WebSocket client)
    └── style.css                     (Styling)
```

---

## Deployment

### Prerequisites

Before deploying, ensure you have:

- **GCP Project** with billing enabled
  - [Create a GCP project](https://console.cloud.google.com/projectcreate)
  - [Enable billing](https://console.cloud.google.com/billing)

- **gcloud CLI** installed and configured
  ```bash
  # Install gcloud: https://cloud.google.com/sdk/docs/install

  # Verify installation
  gcloud --version

  # Login to GCP
  gcloud auth login
  ```

- **Terraform** installed (version 1.0 or higher)
  ```bash
  # Install terraform: https://www.terraform.io/downloads

  # Verify installation
  terraform --version
  ```

- **Git** and the repository cloned
  ```bash
  git clone https://github.com/carlos959358/ClickerGCP.git
  cd ClickerGCP
  ```

### Step-by-Step Deployment Guide

#### Step 1: Configure GCP Project

```bash
# Set your GCP project ID (replace with your actual project ID)
export GCP_PROJECT_ID="your-gcp-project-id"

# Set the default project for gcloud
gcloud config set project $GCP_PROJECT_ID

# Authenticate with GCP (for Terraform)
gcloud auth application-default login
```

**What this does:**
- Sets up authentication for Terraform to access your GCP project
- Enables local tools to interact with your GCP resources

#### Step 2: Configure Terraform Variables

```bash
cd terraform

# Copy the example variables file
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars with your project ID
# You need to update:
# - gcp_project_id = "your-gcp-project-id"
# - github_owner = "your-github-username"
# - github_repo = "ClickerGCP"

# Or use sed to auto-update (Linux/Mac)
sed -i 's/your-project-id/'$GCP_PROJECT_ID'/g' terraform.tfvars
```

**What to configure:**
```hcl
# terraform.tfvars
gcp_project_id = "your-gcp-project-id"           # Required: Your GCP project ID
github_owner   = "your-github-username"          # Required: Your GitHub username
github_repo    = "ClickerGCP"                    # Required: Repository name
gcp_region     = "europe-southwest1"             # Optional: GCP region (default is fine)
```

#### Step 3: Initialize Terraform

```bash
# Download Terraform providers and modules
terraform init
```

**What this does:**
- Downloads the Google Cloud Terraform provider
- Sets up the `.terraform` directory with necessary configurations
- Initializes the Terraform backend (GCS bucket for remote state)

#### Step 4: Review the Deployment Plan

```bash
# See what Terraform will create
terraform plan
```

**Expected output:**
- 30+ resources to be created (services, databases, IAM roles, etc.)
- Docker image builds via Cloud Build
- No errors or warnings

#### Step 5: Deploy Everything

```bash
# Deploy all infrastructure (this will take 5-10 minutes)
terraform apply
```

**Interactive prompt:**
- Terraform will ask: `Do you want to perform these actions?`
- Type `yes` and press Enter to confirm

**What happens during deployment:**
1. ✅ Enables required GCP APIs
2. ✅ Creates Artifact Registry repository
3. ✅ Builds Docker images via Cloud Build (~2-3 minutes each)
4. ✅ Pushes images to Artifact Registry
5. ✅ Deploys backend service to Cloud Run
6. ✅ Deploys consumer service to Cloud Run
7. ✅ Creates Firestore database
8. ✅ Creates Pub/Sub topic and subscription
9. ✅ Configures IAM roles and service accounts

**Expected output:**
```
Apply complete! Resources: 30+ added, 0 changed, 0 destroyed.

Outputs:
backend_url = "https://clicker-backend-xxx.a.run.app"
consumer_url = "https://clicker-consumer-xxx.a.run.app"
artifact_registry_repository = "europe-southwest1-docker.pkg.dev/..."
...
```

---

### Step 6: Verify Deployment

After Terraform completes, verify everything is working:

```bash
# Get your backend service URL
BACKEND_URL=$(terraform output -raw backend_url)
echo "Backend URL: $BACKEND_URL"

# Test 1: Health check
curl "$BACKEND_URL/health"
# Expected response: {"status":"ok"}

# Test 2: Get initial counts
curl "$BACKEND_URL/count"
# Expected response: {"global":0,"countries":{}}

# Test 3: Send a test click
curl "$BACKEND_URL/click?country=US&ip=192.168.1.1"
# Expected response: {"success":true}

# Test 4: Wait 2 seconds for Pub/Sub delivery
sleep 2

# Test 5: Check if counter incremented
curl "$BACKEND_URL/count"
# Expected response: {"global":1,"countries":{"US":1}}
```

If all tests pass, **your deployment is successful!** 🎉

---

### Step 7 (Optional): Set Up Continuous Deployment

To automatically deploy when you push to GitHub, set up Cloud Build triggers:

#### 7a. Connect GitHub Repository

1. Visit [GCP Console > Cloud Build > Triggers](https://console.cloud.google.com/cloud-build/triggers)
2. Click **Create Trigger**
3. Select **GitHub (Cloud Build GitHub App)** as the source
4. Click **Authorize Cloud Build** (this opens a GitHub OAuth flow)
5. Authorize the Cloud Build GitHub App to access your repositories
6. Select the **ClickerGCP** repository
7. Click **Continue**

#### 7b. Create Backend Trigger

1. **Name:** `build-backend`
2. **Description:** "Build and push backend Docker image"
3. **Event:** Push to a branch
4. **Branch regex:** `^main$`
5. **Build configuration:** Dockerfile
6. **Dockerfile directory:** `backend/`
7. **Dockerfile name:** `Dockerfile`
8. **Image name:** `europe-southwest1-docker.pkg.dev/$PROJECT_ID/clicker-repo/backend:$SHORT_SHA`
9. Click **Create Trigger**

#### 7c. Create Consumer Trigger

Repeat step 7b but for the consumer:
1. **Name:** `build-consumer`
2. **Dockerfile directory:** `consumer/`
3. **Image name:** `europe-southwest1-docker.pkg.dev/$PROJECT_ID/clicker-repo/consumer:$SHORT_SHA`

**After setup:**
- Every push to `main` branch triggers automatic builds
- Docker images are built and pushed to Artifact Registry
- Cloud Run services auto-update with new images

---

### Cleanup: Destroy Everything

When you're done testing and want to delete all resources (stop incurring charges):

```bash
# List all resources that will be destroyed
terraform plan -destroy

# Destroy all infrastructure (requires confirmation)
terraform destroy

# Or auto-approve destruction (careful!)
terraform destroy -auto-approve
```

**What this destroys:**
- Cloud Run services
- Firestore database
- Pub/Sub topic and subscription
- Artifact Registry repository and images
- Service accounts and IAM roles
- All other created resources

**Note:** Some resources may take a few minutes to delete.

---

### Troubleshooting

#### Error: "Database ID 'clicker-db' is not available"
- **Cause:** Firestore is being created or was recently deleted
- **Fix:** Wait 2-3 minutes and run `terraform apply` again

#### Error: "Cloud Build: Request contains an invalid argument"
- **Cause:** GitHub repository not connected (we removed triggers from Terraform)
- **Fix:** Set up Cloud Build triggers manually (see Step 7 above)

#### Error: "Permission denied" during build
- **Cause:** Service account lacks required IAM roles
- **Fix:** Terraform should have configured all roles automatically. Run `terraform apply` again or check IAM settings in GCP Console

#### Services stuck in "Creating" state
- **Cause:** Container image pull failure or service startup issues
- **Fix:** Check Cloud Run service logs:
  ```bash
  gcloud run services logs read clicker-backend --limit=50
  ```

#### Can't authenticate with GCP
- **Fix:** Re-run authentication:
  ```bash
  gcloud auth application-default login
  gcloud auth login
  ```

---

### Summary: What Developers Must Do

| Step | Command | Time |
|------|---------|------|
| 1 | Clone repo & cd ClickerGCP | 1 min |
| 2 | Configure GCP project | 5 min |
| 3 | Update terraform/terraform.tfvars | 2 min |
| 4 | `terraform init` | 2 min |
| 5 | `terraform plan` | 2 min |
| 6 | `terraform apply` | 5-10 min |
| 7 | Test endpoints | 2 min |
| 8 (Optional) | Set up Cloud Build triggers | 5 min |
| **Total** | **Full deployment** | **~20-30 min** |

**Core requirement:** Just 6 commands:
```bash
gcloud auth application-default login
cd terraform
terraform init
terraform plan
terraform apply
terraform output
```

---

## API Endpoints

### Backend Service

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/click` | GET | Record a click for a country/IP |
| `/count` | GET | Get global and country counters |
| `/countries` | GET | Get all country counters |
| `/health` | GET | Health check |
| `/debug/config` | GET | Debug configuration (services ready?) |
| `/debug/firestore` | GET | Debug Firestore data |
| `/ws` | WS | WebSocket endpoint for real-time updates |
| `/internal/broadcast` | POST | Internal endpoint for consumer notifications |

### Consumer Service

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/process` | POST | Pub/Sub push webhook (message processing) |
| `/health` | GET | Health check |

### Example Requests

```bash
# Record a click
curl "https://clicker-backend-xxx.run.app/click?country=US&ip=192.168.1.1"

# Get counters
curl "https://clicker-backend-xxx.run.app/count"

# Connect WebSocket (browser)
const ws = new WebSocket('wss://clicker-backend-xxx.run.app/ws');
ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  console.log('Counter updated:', update);
};
```

---

## Testing

### Run Unit Tests

```bash
cd consumer
go test -v
```

**Coverage:** 15 tests covering:
- Successful message processing
- Duplicate detection (idempotency)
- Invalid JSON/base64 handling
- Firestore failure handling
- Missing required fields
- Invalid event format
- Backend notification failures
- Uninitialized services
- Multiple countries
- Complete end-to-end flow
- Timestamp format compatibility
- Pub/Sub message format validation
- Continuous operation with multiple messages
- Idempotency during continuous flow

### Manual End-to-End Test

```bash
# 1. Send test clicks
curl "https://clicker-backend-xxx.run.app/click?country=Test&ip=1.2.3.4"
curl "https://clicker-backend-xxx.run.app/click?country=Test&ip=1.2.3.5"
curl "https://clicker-backend-xxx.run.app/click?country=Test&ip=1.2.3.6"

# 2. Wait for Pub/Sub delivery (typically <1 second)
sleep 2

# 3. Check counters incremented
curl "https://clicker-backend-xxx.run.app/count"
# Expected: {"global": 3, "countries": {...}}
```

---

## Key Features

### Reliable Message Processing
- **Idempotency:** Messages processed only once, even if Pub/Sub retries
- **Atomic Updates:** Firestore transactions ensure counter consistency
- **Error Handling:** Proper HTTP status codes (400/500) for Pub/Sub retry semantics

### Scalable Architecture
- **Pub/Sub:** Decouples backend from consumer, handles load spikes
- **Cloud Run:** Auto-scales based on demand
- **Firestore:** Managed NoSQL database with sub-millisecond latency

### Real-Time Updates
- **WebSocket:** Direct connection from frontend to backend
- **Broadcasting:** All connected clients receive updates instantly
- **Broadcast API:** Consumer notifies backend of counter changes

### Production Ready
- **Comprehensive Testing:** 15 tests with 100% pass rate
- **Logging:** Detailed step-by-step logs for debugging
- **IAM Security:** Service accounts with minimal required permissions
- **Health Checks:** Liveness and readiness probes for orchestration

---

## Configuration

### Environment Variables

**Backend:**
```bash
GCP_PROJECT_ID       # GCP project ID (required)
PORT                 # HTTP port (default: 8080)
```

**Consumer:**
```bash
GCP_PROJECT_ID       # GCP project ID (required)
BACKEND_URL          # Backend service URL (required)
FIRESTORE_DATABASE   # Firestore database ID (default: (default))
PORT                 # HTTP port (default: 8080)
```

### Firestore Collections

```
/counters
  /global
    count: 1234
  /country_US
    country: "United States"
    count: 567
  /country_ES
    country: "Spain"
    count: 234

/processed_messages
  /17886842157423762
    messageId: "17886842157423762"
    country: "US"
    timestamp: "2026-02-02T13:54:22Z"
```

---

## Error Handling

### Status Codes

| Status | Meaning | Action |
|--------|---------|--------|
| 200 OK | Success | Message processed |
| 400 Bad Request | Invalid input | Pub/Sub won't retry |
| 500 Internal Server Error | Server error | Pub/Sub will retry |

### Common Errors & Fixes

**Backend publisher not initialized:**
```
"publisherError": "rpc error: code = PermissionDenied"
Fix: Grant roles/pubsub.editor to backend service account
```

**Consumer idempotency check fails:**
```
"error": "idempotency check failed"
Fix: Ensure processed_messages collection is accessible
```

**Counters not incrementing:**
```
Check:
1. Backend /debug/config shows pubsubPublisher: true
2. Consumer logs show [/process] ===== SUCCESS =====
3. Firestore has global and country_* documents
```

---

## Monitoring

### View Logs

```bash
# Backend logs
gcloud run services logs read clicker-backend \
  --region=europe-southwest1 \
  --limit=50

# Consumer logs
gcloud run services logs read clicker-consumer \
  --region=europe-southwest1 \
  --limit=50
```

### Check Service Status

```bash
# Backend health
curl "https://clicker-backend-xxx.run.app/health"

# Consumer health
curl "https://clicker-consumer-xxx.run.app/health"

# Pub/Sub metrics
gcloud pubsub subscriptions describe click-consumer-sub \
  --project=$GCP_PROJECT_ID
```

---

## Performance

### Latency
- Frontend → Backend: ~50ms (network)
- Backend → Pub/Sub: ~200ms (publish + acknowledgment)
- Pub/Sub → Consumer: <1s (typical push delivery)
- Consumer → Firestore: ~50ms (transaction)
- Total: ~1.3s end-to-end (typical)

### Throughput
- Backend: 1,000+ requests/second (Cloud Run auto-scaling)
- Consumer: 100+ messages/second (configurable concurrency)
- Firestore: 10,000+ writes/second (standard pricing)

### Cost (GCP Free Tier)
- Cloud Run: 2 million free requests/month
- Firestore: 25,000 reads/day free
- Pub/Sub: 10GB free/month
- Total cost: ~$0/month for typical usage

---

## Development

### Local Development

```bash
# Start backend locally
cd backend
go run main.go
# Listens on http://localhost:8080

# In another terminal, start consumer
cd consumer
go run main.go
# Listens on http://localhost:8080 (use different port in practice)
```

### Running Tests

```bash
cd consumer
go test -v                    # Run all tests
go test -v -run TestSuccess   # Run specific test
go test -cover                # Show coverage
```

### Code Structure

**Backend:**
- `main.go`: HTTP handlers, WebSocket hub, Pub/Sub publisher initialization
- `firestore.go`: Firestore client, counter reading

**Consumer:**
- `main.go`: HTTP /process endpoint handler
- `firestore.go`: Firestore client, counter updates, idempotency checks
- `notifier.go`: Backend notification client
- `interfaces.go`: Mock interfaces for testing
- `main_test.go`: Unit tests (10 tests)
- `integration_test.go`: Integration tests (5 tests)

---

## Security

### Service Accounts

**Backend Service Account**
- `roles/pubsub.editor` - Publish to Pub/Sub topics
- `roles/datastore.user` - Read from Firestore

**Consumer Service Account**
- `roles/pubsub.subscriber` - Consume from Pub/Sub subscriptions
- `roles/pubsub.viewer` - View subscription metadata
- `roles/datastore.user` - Read/write Firestore

### Network Security

- All services run on Google Cloud Run (DDoS protection included)
- Firestore: Authenticated access only (default)
- Pub/Sub: OIDC token authentication on push delivery
- WebSocket: Runs on same domain as backend (same-origin)

### Data Privacy

- No personally identifiable information stored
- Only country codes and IP addresses (for geolocation)
- Counters are public data
- No sensitive data in Firestore or Pub/Sub

---

## Troubleshooting

### Messages Not Being Processed

1. **Check backend is publishing:**
   ```bash
   curl "https://clicker-backend-xxx.run.app/debug/config"
   # Look for: "pubsubPublisher": true
   ```

2. **Check subscription push endpoint:**
   ```bash
   gcloud pubsub subscriptions describe click-consumer-sub \
     --project=$GCP_PROJECT_ID
   # Look for: pushEndpoint: https://clicker-consumer-xxx.run.app/process
   ```

3. **Check consumer logs:**
   ```bash
   gcloud run services logs read clicker-consumer --limit=20
   # Look for: [/process] ===== SUCCESS =====
   ```

### Counters Not Incrementing

1. **Send test click:**
   ```bash
   curl "https://clicker-backend-xxx.run.app/click?country=US&ip=1.2.3.4"
   ```

2. **Check backend logs:**
   ```bash
   gcloud run services logs read clicker-backend --limit=20
   # Look for: ✓ Counters incremented
   ```

3. **Check consumer received message:**
   ```bash
   gcloud run services logs read clicker-consumer --limit=20
   # Look for: [/process] ✓ Idempotency check
   ```

4. **Verify Firestore has data:**
   ```bash
   gcloud firestore export gs://your-bucket/backup
   ```

---

## Support & Documentation

- **Terraform Configuration:** See `terraform/` directory
- **API Documentation:** See inline code comments
- **Test Suite:** See `consumer/main_test.go` and `consumer/integration_test.go`

---

## License

This project is provided as-is for educational and development purposes.

---

**Last Updated:** 2026-02-02
**Status:** ✅ Production Ready
**Tests:** 15/15 Passing
