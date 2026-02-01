# Deployment Guide

Complete instructions for deploying Clicker GCP to your own Google Cloud Project.

## Prerequisites

Before you start, ensure you have:

1. **Google Cloud Account**
   - Go to [Google Cloud Console](https://console.cloud.google.com)
   - Create a new project
   - Enable billing for the project

2. **Install Required Tools**
   ```bash
   # Install gcloud CLI
   curl https://sdk.cloud.google.com | bash
   exec -l $SHELL  # Reload shell
   gcloud init

   # Install Terraform
   # macOS
   brew install terraform
   # Linux/WSL
   curl https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
   sudo apt-add-repository "deb https://apt.releases.hashicorp.com $(lsb_release -cs) main"
   sudo apt-get update && sudo apt-get install terraform

   # Install Docker
   # Follow: https://docs.docker.com/get-docker/
   ```

3. **Clone the Repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/ClickerGCP.git
   cd ClickerGCP
   ```

## Step 1: Configure GCP Project

```bash
# Set your project ID (replace with your actual project ID)
export GCP_PROJECT_ID="your-gcp-project-id"

# Configure gcloud
gcloud config set project $GCP_PROJECT_ID

# Enable required APIs
gcloud services enable \
  cloudrun.googleapis.com \
  firestore.googleapis.com \
  pubsub.googleapis.com \
  artifactregistry.googleapis.com \
  container.googleapis.com
```

## Step 2: Configure Terraform Variables

```bash
# Copy the example file
cp terraform.tfvars.example terraform.tfvars

# Edit with your settings
nano terraform.tfvars
```

Update the following in `terraform.tfvars`:

```hcl
gcp_project_id = "your-actual-project-id"
gcp_region = "us-central1"  # Change if desired
```

## Step 3: Deploy Infrastructure

```bash
# Set environment variable for deployment
export GCP_PROJECT_ID="your-gcp-project-id"

# Run deployment script
./scripts/deploy.sh
```

The script will:
1. Create Artifact Registry repository
2. Build Docker images for backend and consumer
3. Push images to Artifact Registry
4. Deploy Cloud Run services with Terraform
5. Output your service URLs

## Step 4: Initialize Database

```bash
# Initialize Firestore collections
./scripts/init-firestore.sh
```

## Step 5: Verify Deployment

### Check Cloud Run Services

```bash
gcloud run services list --region=$(gcloud config get compute/region)
```

### Test API Endpoints

```bash
# Get the backend URL from deployment output
BACKEND_URL="https://YOUR_BACKEND_URL.run.app"

# Health check
curl $BACKEND_URL/health

# Get current count
curl $BACKEND_URL/count

# Send a test click
curl -X POST $BACKEND_URL/click
```

### Check Firestore Data

```bash
gcloud firestore documents list --collection=counters
```

## Configuration Options

### Scaling Settings

Edit `terraform.tfvars` to adjust instance scaling:

```hcl
# Backend service scaling
backend_min_instances  = 1    # Minimum instances (0 for cold start)
backend_max_instances  = 10   # Maximum instances

# Consumer service scaling
consumer_min_instances = 1
consumer_max_instances = 5
```

### Resource Allocation

```hcl
backend_memory  = "256Mi"   # Memory per backend instance
consumer_memory = "256Mi"   # Memory per consumer instance
```

### Region

Change deployment region:

```hcl
gcp_region = "us-central1"  # Or any other GCP region
```

## Security Configuration

### 1. Set Up Remote Terraform State

Store state securely in Google Cloud Storage:

```bash
# Create GCS bucket for Terraform state
BUCKET_NAME="$GCP_PROJECT_ID-terraform-state"
gsutil mb gs://$BUCKET_NAME
gsutil versioning set on gs://$BUCKET_NAME

# Enable versioning
gsutil logging set on -b gs://$BUCKET_NAME gs://$BUCKET_NAME
```

Create `terraform/backend.tf`:

```hcl
terraform {
  backend "gcs" {
    bucket = "YOUR_PROJECT_ID-terraform-state"
    prefix = "clicker"
  }
}
```

### 2. Restrict Cloud Run Services

```bash
# Backend should be public, consumer internal only
gcloud run services update clicker-consumer \
  --no-allow-unauthenticated \
  --region=$(gcloud config get compute/region)
```

### 3. Enable Audit Logging

```bash
gcloud logging sinks create cloud-run-audit \
  logging.googleapis.com/organizations/YOUR_ORG_ID/logs \
  --log-filter='resource.type=cloud_run_resource'
```

## Monitoring

### View Logs

```bash
# Backend logs
gcloud run logs read clicker-backend --limit=50

# Consumer logs
gcloud run logs read clicker-consumer --limit=50
```

### Monitor Costs

```bash
# View current billing
gcloud billing accounts list
gcloud billing accounts describe ACCOUNT_ID --format=json
```

## Troubleshooting

### Common Errors

**"Project not found"**
```bash
# Verify project ID
gcloud config list
# Set correct project
export GCP_PROJECT_ID="correct-project-id"
```

**"Permission denied" errors**
```bash
# Ensure you have Editor role on the project
gcloud projects get-iam-policy $GCP_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:YOUR_EMAIL"
```

**Firestore already exists**
```bash
# This is normal if you've deployed before
# It won't affect existing data
```

**Pub/Sub messages not processing**
- Check consumer service is running: `gcloud run services describe clicker-consumer`
- View logs: `gcloud run logs read clicker-consumer`
- Verify IAM roles: `gcloud projects get-iam-policy $GCP_PROJECT_ID`

## Cleanup

To remove all resources and avoid charges:

```bash
# Destroy Terraform resources
cd terraform
terraform destroy -auto-approve

# Delete Artifact Registry repository (optional)
gcloud artifacts repositories delete clicker-repo --location=$(gcloud config get compute/region)

# Delete Firestore database (optional)
gcloud firestore databases delete --database=clicker-db
```

## Next Steps

1. **Customize Frontend**
   - Edit `backend/static/index.html` for branding
   - Modify `backend/static/css/style.css` for styling

2. **Extend Functionality**
   - Add user authentication
   - Implement leaderboards
   - Add new game mechanics

3. **Production Hardening**
   - Enable VPC Service Controls
   - Implement Cloud Armor
   - Set up Cloud CDN
   - Enable Binary Authorization

4. **Monitoring & Alerts**
   - Configure Cloud Monitoring
   - Set up alert policies
   - Create custom dashboards

## Support

For issues or questions:
1. Check [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)
2. Review [GCP documentation](https://cloud.google.com/docs)
3. Open an issue on GitHub

## Additional Resources

- [Google Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Terraform Google Provider](https://registry.terraform.io/providers/hashicorp/google/latest/docs)
- [Go Client Libraries](https://cloud.google.com/go/docs)
- [GCP Pricing](https://cloud.google.com/pricing)
