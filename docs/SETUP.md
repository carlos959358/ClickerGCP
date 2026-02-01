# Clicker GCP - Setup Guide

This guide walks you through deploying the Clicker application to Google Cloud Platform.

## Prerequisites

- GCP project with billing enabled
- `gcloud` CLI configured with appropriate credentials
- `terraform` installed (v1.0+)
- `docker` installed and authenticated with your container registry
- Go 1.22+ (optional, only if building locally)

## Step 1: Set Environment Variables

```bash
export GCP_PROJECT_ID="your-gcp-project-id"
export GCP_REGION="us-central1"  # or your preferred region
export BACKEND_SERVICE_NAME="clicker-backend"
export CONSUMER_SERVICE_NAME="clicker-consumer"
export ARTIFACT_REPO="clicker-repo"
```

## Step 2: Verify GCP Setup

Ensure your GCP project is properly configured:

```bash
gcloud config set project $GCP_PROJECT_ID
gcloud config set compute/region $GCP_REGION

# Verify authentication
gcloud auth list
gcloud projects describe $GCP_PROJECT_ID
```

## Step 3: Deploy Infrastructure

The deployment script handles all necessary steps:

```bash
cd /path/to/ClickerGCP
./scripts/deploy.sh
```

This will:
1. Create/validate Artifact Registry repository
2. Build and push Docker images
3. Run Terraform to create all GCP resources
4. Output service URLs

## Step 4: Initialize Firestore

After deployment, initialize the Firestore database:

```bash
./scripts/init-firestore.sh
```

## Step 5: Configure Frontend

Update the frontend configuration with your backend URL:

Edit `frontend/js/app.js` and set `CONFIG.BACKEND_URL`:

```javascript
const CONFIG = {
    BACKEND_URL: 'https://your-backend-url.run.app',
    // ...
};
```

Or it will auto-detect if running from the same domain.

## Step 6: Deploy Frontend

### Option A: Cloud Storage + Cloud CDN (Recommended)

```bash
# Create a Cloud Storage bucket
gsutil mb gs://$GCP_PROJECT_ID-clicker

# Upload frontend files
gsutil -m cp -r frontend/* gs://$GCP_PROJECT_ID-clicker/

# Set up Cloud CDN and make bucket public
gsutil iam ch allUsers:objectViewer gs://$GCP_PROJECT_ID-clicker
```

### Option B: Serve from Backend

Copy the frontend files to the backend service and serve them:

```bash
# This would require modifying the backend to serve static files
# Not implemented in the current version
```

## Verification

### Test Backend Health

```bash
BACKEND_URL=$(cd terraform && terraform output -raw backend_url)

# Health check
curl $BACKEND_URL/health

# Get current counts
curl $BACKEND_URL/count

# Send a test click
curl -X POST $BACKEND_URL/click
```

### Test WebSocket Connection

```bash
# Using websocat (install with: cargo install websocat)
BACKEND_URL=$(cd terraform && terraform output -raw backend_url)
WS_URL=$(echo $BACKEND_URL | sed 's/https:/wss:/' | sed 's/http:/ws:/')

websocat $WS_URL/ws
```

### Monitor Logs

```bash
# Backend logs
gcloud run logs read $BACKEND_SERVICE_NAME --limit=50

# Consumer logs
gcloud run logs read $CONSUMER_SERVICE_NAME --limit=50

# Firestore operations
gcloud logging read "resource.type=cloud_firestore_database" --limit=50 --format=json
```

### Check Pub/Sub Topic

```bash
# List messages (note: messages are auto-deleted after 600s)
gcloud pubsub subscriptions pull click-consumer-sub --auto-ack --limit=10
```

## Load Testing

Test the application with multiple concurrent clicks:

```bash
# Using 'hey' (install with: go install github.com/rakyll/hey@latest)
BACKEND_URL=$(cd terraform && terraform output -raw backend_url)

hey -n 1000 -c 10 -m POST $BACKEND_URL/click
```

This sends 1000 requests with 10 concurrent connections.

## Troubleshooting

### Cannot connect to backend

1. Verify Cloud Run services are running:
   ```bash
   gcloud run services list
   ```

2. Check service URLs:
   ```bash
   cd terraform && terraform output
   ```

3. Verify IAM permissions:
   ```bash
   gcloud run services get-iam-policy $BACKEND_SERVICE_NAME
   ```

### Firestore errors

1. Verify database exists:
   ```bash
   gcloud firestore databases describe --database=$FIRESTORE_DATABASE
   ```

2. Check collection structure:
   ```bash
   gcloud firestore documents list counters
   ```

### Pub/Sub not working

1. Verify topic exists:
   ```bash
   gcloud pubsub topics list
   ```

2. Check subscription:
   ```bash
   gcloud pubsub subscriptions describe click-consumer-sub
   ```

3. Monitor consumer service logs for errors

### Cold starts

The application may take 1-3 seconds to start on first request. To keep services warm:

1. Set minimum instances in Terraform (increases cost):
   ```
   backend_min_instances = 1
   ```

2. Or set up periodic health check using Cloud Scheduler

## Cleanup

To remove all resources and stop incurring charges:

```bash
# Destroy Terraform resources
cd terraform
terraform destroy -auto-approve

# Delete Artifact Registry images
gcloud artifacts repositories delete $ARTIFACT_REPO --location=$GCP_REGION

# Delete Cloud Storage bucket (if deployed)
gsutil -m rm -r gs://$GCP_PROJECT_ID-clicker
```

## Cost Estimation

With default settings (low traffic < 1K clicks/minute):

- **Cloud Run (Backend)**: ~$5-10/month
- **Cloud Run (Consumer)**: ~$2-5/month
- **Pub/Sub**: Free (under quota)
- **Firestore**: Free (under quota)
- **Total**: ~$10-15/month

## Next Steps

- Set up [Cloud Monitoring](https://cloud.google.com/monitoring) dashboards
- Enable [Cloud Armor](https://cloud.google.com/armor) for DDoS protection
- Configure [Cloud CDN](https://cloud.google.com/cdn) for frontend caching
- Implement [Firestore real-time listeners](https://cloud.google.com/firestore/docs/query-data/listen) for instant updates

## Support

For issues or questions:

1. Check [Cloud Run documentation](https://cloud.google.com/run/docs)
2. Review [Firestore documentation](https://cloud.google.com/firestore/docs)
3. Check [Pub/Sub documentation](https://cloud.google.com/pubsub/docs)
4. Review service logs in Cloud Console
