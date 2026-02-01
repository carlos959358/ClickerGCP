# Clicker GCP 🎮

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Terraform](https://img.shields.io/badge/Terraform-v1.0+-844FBA?style=flat&logo=terraform)](https://www.terraform.io)
[![Google Cloud](https://img.shields.io/badge/Google%20Cloud-Platform-4285F4?style=flat&logo=google-cloud)](https://cloud.google.com)

A distributed, real-time global counter game built on **Google Cloud Platform** with serverless architecture, WebSocket synchronization, and geolocation tracking.

> **Watch clicks happen in real-time from users around the world.** Simple concept. Powerful architecture.

## ✨ Features

- 🌍 **Real-time Global Counter** - See clicks update instantly across all connected users
- 📍 **Geolocation Tracking** - Automatic country detection for each click
- ⚡ **Serverless & Auto-scaling** - Cloud Run handles millions of requests automatically
- 🔄 **Event-Driven Architecture** - Pub/Sub message queue for reliable click processing
- 🗄️ **NoSQL Database** - Firestore for fast, scalable data storage
- 🔌 **WebSocket Real-time Sync** - Instant updates without polling
- 🐳 **Containerized** - Docker multi-stage builds for optimized deployment
- 🏗️ **Infrastructure as Code** - Complete Terraform configuration included
- 📚 **Production-Ready** - Security hardening guides and best practices included

## 🏛️ Architecture

```
┌─────────────────────┐
│   Browser Client    │
│   (Frontend HTML)   │
└──────────┬──────────┘
           │ WebSocket & REST API
           ▼
┌──────────────────────────┐
│  Cloud Run Backend       │ ◄─── Serves static frontend
│  - HTTP/WebSocket        │
│  - Pub/Sub Publisher     │
│  - Firestore Reader      │
└──────────┬───────────────┘
           │
           ├──► Pub/Sub Topic (click-events)
           │           ▼
           │    ┌─────────────────────┐
           │    │ Cloud Run Consumer  │
           │    │ - Message Processor │
           │    │ - Firestore Writer  │
           │    │ - Backend Notifier  │
           │    └────────┬────────────┘
           │             │
           └─────────────┼──────────────┐
                         ▼              ▼
                    ┌──────────────────────────┐
                    │  Firestore Database      │
                    │  - Global Counter        │
                    │  - Country-wise Counts   │
                    └──────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- GCP Account with billing enabled
- `gcloud` CLI ([install](https://cloud.google.com/sdk/docs/install))
- Terraform 1.0+ ([install](https://www.terraform.io/downloads))
- Docker ([install](https://docs.docker.com/get-docker/))
- Git
- GitHub repository access (for Cloud Build integration)

### 1. Clone & Configure

```bash
# Clone the repository
git clone https://github.com/carlos959358/ClickerGCP.git
cd ClickerGCP

# Create Terraform configuration
mkdir -p terraform
cat > terraform/terraform.tfvars <<EOF
gcp_project_id = "your-gcp-project-id"
gcp_region     = "europe-southwest1"
github_owner   = "your-github-username"
github_repo    = "ClickerGCP"
EOF
```

### 2. Initialize Infrastructure (Firestore, Pub/Sub, Cloud Run)

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

This sets up:
- ✅ Firestore database
- ✅ Pub/Sub topic & subscription
- ✅ Artifact Registry repository
- ✅ Cloud Run services (backend & consumer)
- ✅ Service accounts & IAM roles

**Note:** Cloud Run services will be in a pending state until Docker images are built and pushed.

### 3. Build & Push Docker Images

**Option A: Via Cloud Build (automatic on push)**

```bash
git push origin main
```

Cloud Build will automatically:
- Build both Docker images
- Push to Artifact Registry
- Update Cloud Run services

Monitor the build:
```bash
gcloud builds log --stream
```

**Option B: Manual Docker build**

```bash
# Authenticate with Artifact Registry
gcloud auth configure-docker europe-southwest1-docker.pkg.dev

# Get your project ID
PROJECT_ID=$(gcloud config get-value project)

# Build and push backend
docker build -t europe-southwest1-docker.pkg.dev/$PROJECT_ID/clicker-repo/backend:latest ./backend
docker push europe-southwest1-docker.pkg.dev/$PROJECT_ID/clicker-repo/backend:latest

# Build and push consumer
docker build -t europe-southwest1-docker.pkg.dev/$PROJECT_ID/clicker-repo/consumer:latest ./consumer
docker push europe-southwest1-docker.pkg.dev/$PROJECT_ID/clicker-repo/consumer:latest
```

### 4. Access Your App

Once images are deployed, your services are live:

```bash
# Get backend URL
gcloud run services describe clicker-backend --region=europe-southwest1 --format='value(status.url)'

# Get consumer URL
gcloud run services describe clicker-consumer --region=europe-southwest1 --format='value(status.url)'
```

Or from Terraform outputs:
```bash
cd terraform
terraform output backend_url
terraform output consumer_url
```

Open the backend URL in your browser and start clicking! 🖱️

## 🔄 Cloud Build CI/CD Pipeline

Cloud Build is now connected to your GitHub repository! When you push to the `main` branch, Cloud Build automatically builds and deploys your application.

### Setting Up Your Build Trigger

If you haven't created the trigger yet:

1. Go to [Cloud Build Triggers](https://console.cloud.google.com/cloud-build/triggers)
2. Click **Create Trigger**
3. Fill in:
   - **Name:** `clicker-main-trigger`
   - **Event:** Push to a branch
   - **Repository:** Select `ClickerGCP`
   - **Branch:** `^main$`
   - **Build configuration:** Cloud Build configuration file
   - **Location:** `/cloudbuild.yaml`
4. Click **Create**

Your trigger is now active and will run on every push to `main`!

### How It Works

When you push to `main`, Cloud Build automatically:

1. **Builds Docker images** - Backend and consumer services
2. **Pushes to Registry** - Images go to Artifact Registry
3. **Runs Terraform** - Applies infrastructure changes
4. **Deploys Services** - Updates Cloud Run services

### Cloud Build Configuration

**Root `cloudbuild.yaml`:**
```yaml
steps:
  1. Build backend Docker image
  2. Push backend to Artifact Registry
  3. Build consumer Docker image
  4. Push consumer to Artifact Registry
  5. Initialize Terraform
  6. Validate Terraform
  7. Plan Terraform changes
  8. Apply Terraform (deploy)
```

### Testing Your Setup

#### Option 1: Push to Main

```bash
# Make a change
echo "# Test" >> README.md

# Push to main
git add README.md
git commit -m "Trigger Cloud Build"
git push origin main
```

View the build in Cloud Build Console or via CLI:

```bash
gcloud builds list --limit=5
gcloud builds log BUILD_ID --stream
```

#### Option 2: Manual Build

```bash
# Manually trigger a build
gcloud builds submit --config=cloudbuild.yaml

# Watch it
gcloud builds log --stream
```

### Monitoring Builds

**View build history:**
```bash
gcloud builds list --limit=10
```

**Watch a specific build:**
```bash
gcloud builds log BUILD_ID --stream
```

**Open Cloud Build Console:**
```bash
# Open in browser
gcloud builds list --filter="status=QUEUED OR status=WORKING" --format="value(id)" | head -1 | xargs -I {} echo "https://console.cloud.google.com/cloud-build/builds/{PROJECT_ID}"
```

Or visit: https://console.cloud.google.com/cloud-build/builds

### Troubleshooting

**Build not triggering on push:**
1. Go to [Cloud Build Triggers](https://console.cloud.google.com/cloud-build/triggers)
2. Verify trigger is enabled
3. Check branch pattern is `^main$`
4. Ensure you're pushing to `main` branch

**Build failures:**
- Click the failed build in Cloud Build Console
- View full logs under "Build Details"
- Common issues:
  - Missing Firestore database - run `terraform apply` first
  - Missing images - check if previous builds succeeded
  - Permission errors - check Cloud Build service account has required IAM roles

**Images not pushing:**
- Verify Artifact Registry repository exists:
  ```bash
  gcloud artifacts repositories list
  ```
- Check Cloud Build service account has `artifactregistry.writer` role
- View build logs for specific error messages

## 📁 Project Structure

```
ClickerGCP/
├── backend/                   # Go HTTP server + WebSocket
│   ├── main.go                # Server, API endpoints, WebSocket
│   ├── firestore.go           # Database operations
│   ├── Dockerfile             # Multi-stage build
│   └── static/                # Frontend assets
│       ├── index.html         # SPA interface
│       ├── js/app.js          # Click handler, WebSocket client
│       └── css/style.css      # Responsive styling
│
├── consumer/                 # Go Pub/Sub message processor
│   ├── main.go               # HTTP server, message handler
│   ├── firestore.go          # Counter update logic
│   ├── notifier.go           # Backend notification client
│   └── Dockerfile
│
├── terraform/               # Infrastructure as Code
│   ├── main.tf              # Provider configuration
│   ├── variables.tf         # Configuration variables
│   ├── cloudrun.tf          # Cloud Run services
│   ├── iam.tf               # Service accounts & permissions
│   ├── pubsub.tf            # Pub/Sub topic & subscription
│   └── firestore.tf         # Firestore database
│
├── scripts/                # Deployment automation
│   ├── deploy.sh           # Main deployment script
│   └── init-firestore.sh   # Database initialization
│
├── docs/ # documentation
│   ├── DEPLOYMENT_GUIDE.md
│   ├── SECURITY_CHECKLIST.md
│   └── ARCHITECTURE.md
│
└── README.md
```

## 🔧 Configuration

### Terraform Variables (`terraform.tfvars`)

```hcl
# Required
gcp_project_id = "your-project-id"

# Optional (defaults shown)
gcp_region              = "us-central1"
backend_memory          = "256Mi"
consumer_memory         = "256Mi"
backend_min_instances   = 1
backend_max_instances   = 10
consumer_min_instances  = 1
consumer_max_instances  = 5
```

See `terraform.tfvars.example` for all available options.

### Environment Variables (`.env`)

```bash
GCP_PROJECT_ID=your-project-id
GCP_REGION=us-central1
FIRESTORE_DATABASE=clicker
```

Create `.env` from template:
```bash
cp .env.example .env
nano .env
```

## 🌍 How It Works

### Click Flow

1. **User clicks** the button in the browser
2. **Frontend sends** click event to backend `/click` endpoint
3. **Backend publishes** click message to Pub/Sub topic
4. **Consumer service** receives message from Pub/Sub
5. **Consumer updates** Firestore counters (global + country-specific)
6. **Consumer notifies** backend via `/internal/broadcast` endpoint
7. **Backend broadcasts** via WebSocket to all connected clients
8. **Frontend updates** UI in real-time with new count

### Geolocation

User's IP address is automatically detected and mapped to country:
- **Primary API**: `ipapi.co` (fast, reliable)
- **Fallback API**: `ip-api.com` (backup if primary fails)
- Clicks are counted both globally and per-country

## 📊 API Endpoints

### Public Endpoints

```bash
# Health check
curl https://YOUR_BACKEND_URL/health

# Get current counts
curl https://YOUR_BACKEND_URL/count

# Record a click (increments counter)
curl -X POST https://YOUR_BACKEND_URL/click

# WebSocket endpoint (real-time updates)
ws://YOUR_BACKEND_URL/ws
```

### Internal Endpoints (Backend to Consumer)

```bash
# Consumer broadcasts counter updates
curl -X POST https://YOUR_BACKEND_URL/internal/broadcast \
  -H "Content-Type: application/json" \
  -d '{"global":42,"countries":{"US":10,"ES":32}}'
```

## 🔒 Security

### What's Included

- ✅ **No hardcoded credentials** - All config via environment variables
- ✅ **Service account isolation** - Each service has minimal required permissions
- ✅ **Terraform state isolation** - Store state remotely in GCS (not in repo)
- ✅ **Security documentation** - See `SECURITY_CHECKLIST.md`
- ✅ **Production hardening guide** - Best practices for going live

### What You Should Do

Before production deployment:

1. **Enable VPC Service Controls** - Restrict Firestore/Pub/Sub access
2. **Implement Cloud Armor** - DDoS protection and WAF rules
3. **Restrict Cloud Run ingress** - Limit to authorized sources
4. **Enable audit logging** - Track all API calls and deployments
5. **Use Secret Manager** - For any runtime secrets
6. **Implement Binary Authorization** - Ensure container security

See [SECURITY_CHECKLIST.md](./SECURITY_CHECKLIST.md) for detailed instructions.

## 💾 State Management

### ⚠️ Important: Don't Commit State Files

Terraform state files (`.tfstate`, `.tfstate.backup`) contain sensitive data and are in `.gitignore`. Never commit them.

### Store State Remotely

For team collaboration and safety:

```bash
# Create GCS bucket for Terraform state
BUCKET_NAME="${GCP_PROJECT_ID}-terraform-state"
gsutil mb gs://$BUCKET_NAME
gsutil versioning set on gs://$BUCKET_NAME

# Create terraform/backend.tf
cat > terraform/backend.tf <<EOF
terraform {
  backend "gcs" {
    bucket = "$BUCKET_NAME"
    prefix = "clicker"
  }
}
EOF

# Migrate state
terraform -chdir=terraform init
```

## 📊 Monitoring & Logs

### View Service Logs

```bash
# Backend logs
gcloud run logs read clicker-backend --limit=100

# Consumer logs
gcloud run logs read clicker-consumer --limit=100
```

### Monitor Firestore

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Navigate to **Firestore** → **Data**
3. View collections: `counters`
4. Check read/write metrics in **Monitoring**

### Set Up Alerts

```bash
# Create alert policy for error rate
gcloud alpha monitoring policies create \
  --notification-channels=CHANNEL_ID \
  --display-name="Cloud Run High Error Rate" \
  --condition-display-name="Error rate > 5%"
```

## 💰 Cost Optimization

This project uses **GCP Free Tier** by default:

| Service | Free Tier | Default Usage |
|---------|-----------|---------------|
| Cloud Run | 2M requests/month | ~100K/month (1000 clicks/day) |
| Firestore | 50K reads + 20K writes/month | ~5K reads + 2K writes/month |
| Pub/Sub | 10GB/month | ~1GB/month |
| Storage | 5GB | ~10MB (just code) |

**Estimated monthly cost**: ~$0-2 USD (within free tier)

To reduce costs further:
- Decrease `backend_max_instances` in `terraform.tfvars`
- Decrease `consumer_max_instances`
- Use regional Firestore instead of multi-region
- Enable Cloud Run on-demand (min instances = 0, cold starts OK)

## 🛠️ Local Development

### Build & Run Locally

```bash
# Backend
cd backend
go build -o backend
GCP_PROJECT_ID=your-project-id ./backend

# Consumer (in another terminal)
cd consumer
go build -o consumer
GCP_PROJECT_ID=your-project-id ./consumer
```

### Test Locally

```bash
# Terminal 1: Run backend
go run backend/main.go

# Terminal 2: Send test requests
curl http://localhost:8080/health
curl http://localhost:8080/count
curl -X POST http://localhost:8080/click

# Terminal 3: View Firestore (with emulator)
# Use Google Cloud Firestore Emulator for local development
firebase emulators:start
```

## 🧹 Cleanup

### Remove All Resources

```bash
cd terraform
terraform destroy -auto-approve
```

This removes:
- Cloud Run services
- Firestore database
- Pub/Sub topic & subscription
- Artifact Registry repository
- Service accounts & IAM roles

## 📚 Documentation

- **[DEPLOYMENT_GUIDE.md](./DEPLOYMENT_GUIDE.md)** - Step-by-step deployment with troubleshooting
- **[SECURITY_CHECKLIST.md](./SECURITY_CHECKLIST.md)** - Security verification and hardening
- **[CLEANUP_SUMMARY.md](./CLEANUP_SUMMARY.md)** - What was removed and why

## 🤝 Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Test thoroughly
5. Commit with clear messages
6. Push to your branch
7. Open a Pull Request

## 📄 License

This project is licensed under the **MIT License** - see [LICENSE](./LICENSE) file for details.

## ⚖️ Disclaimer

This is a demonstration project. Before production deployment:
- Review all security configurations
- Implement proper authentication/authorization if needed
- Set up comprehensive monitoring and alerting
- Configure automated backups and disaster recovery
- Perform load testing with realistic traffic
- Review [Google Cloud Security Best Practices](https://cloud.google.com/security/best-practices)

## 🆘 Troubleshooting

### Common Issues

**"Project not found" error**
```bash
# Verify project ID
echo $GCP_PROJECT_ID
# Set if empty
export GCP_PROJECT_ID="your-project-id"
```

**Permission denied errors**
```bash
# Check your IAM role
gcloud projects get-iam-policy $GCP_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:YOUR_EMAIL"

# You need Editor or equivalent role
```

**Firestore already exists**
```bash
# This is normal - it won't affect existing data
# Just continue with the deployment
```

**WebSocket connection fails**
- Check browser console for errors
- Verify backend service is running: `gcloud run services describe clicker-backend`
- Check logs: `gcloud run logs read clicker-backend`

**Pub/Sub messages not processing**
- Verify consumer service is running
- Check consumer has correct IAM roles
- View consumer logs: `gcloud run logs read clicker-consumer`
- Verify Pub/Sub subscription is active

For detailed troubleshooting, see [DEPLOYMENT_GUIDE.md](./DEPLOYMENT_GUIDE.md#troubleshooting).

## 📞 Support

- 📖 Read the [DEPLOYMENT_GUIDE.md](./DEPLOYMENT_GUIDE.md)
- 🔍 Check [SECURITY_CHECKLIST.md](./SECURITY_CHECKLIST.md)
- 🐛 Open an [GitHub Issue](https://github.com/YOUR_USERNAME/ClickerGCP/issues)
- 📚 Review [Google Cloud Documentation](https://cloud.google.com/docs)

## 🎯 What You'll Learn

Building this project teaches you:

- **Serverless Architecture** - Cloud Run auto-scaling and cold starts
- **Event-Driven Design** - Pub/Sub message processing patterns
- **Real-time Communication** - WebSocket connections at scale
- **Infrastructure as Code** - Terraform for GCP resources
- **Go Best Practices** - HTTP servers, concurrency, error handling
- **Database Design** - Firestore collections and queries
- **Security** - Service accounts, IAM roles, secret management
- **DevOps** - Docker containerization and CI/CD concepts

## 🚀 Next Steps

1. **Customize the frontend** - Edit `backend/static/index.html`
2. **Add authentication** - Implement user accounts
3. **Create leaderboards** - Track top countries
4. **Add game mechanics** - Combos, power-ups, achievements
5. **Scale to production** - Enable all security features

## 📈 Project Stats

- **Language**: Go 1.22+
- **Frontend**: HTML5 + Vanilla JavaScript
- **Infrastructure**: Terraform (55+ resources)
- **Cloud Platform**: Google Cloud Platform
- **License**: MIT
- **Code Size**: ~1500 lines (backend + consumer)

---

<div align="center">

**Built with ❤️ on Google Cloud Platform**

**[⭐ Star this repo](https://github.com/YOUR_USERNAME/ClickerGCP) if you find it helpful!**

[Report Bug](https://github.com/YOUR_USERNAME/ClickerGCP/issues) · [Request Feature](https://github.com/YOUR_USERNAME/ClickerGCP/issues) · [View Demo](#quick-start)

</div>
