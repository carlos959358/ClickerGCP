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

### 1. Clone & Configure

```bash
# Clone the repository
git clone https://github.com/carlos959358/ClickerGCP.git
cd ClickerGCP

# Create configuration from template
cp terraform.tfvars.example terraform.tfvars

# Edit with your GCP project ID
nano terraform.tfvars
```

Update `terraform.tfvars`:
```hcl
gcp_project_id = "your-actual-gcp-project-id"
gcp_region     = "us-central1"  # or your preferred region
```

### 2. Deploy to GCP

```bash
# This will build, push, and deploy everything
./scripts/deploy.sh
```

The script will:
- ✅ Enable required GCP APIs
- ✅ Create Artifact Registry repository
- ✅ Build and push Docker images
- ✅ Deploy Cloud Run services
- ✅ Set up Firestore database
- ✅ Output your service URLs

### 3. Access Your App

Once deployment completes, open the backend URL in your browser:

```
https://clicker-backend-XXXXXXX.a.run.app
```

Start clicking! 🖱️

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
