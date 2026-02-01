# Quick Start: Deploy GCP Free Tier Optimization

## 5-Minute Setup

### Prerequisites
```bash
# Ensure you have:
# - gcloud CLI installed and configured
# - terraform installed
# - Access to GCP project

export GCP_PROJECT_ID="your-project-id"
cd /home/carlos/Desktop/DevProjects/ClickerGCP
```

---

## Phase 1: Cloud Run Optimization (15 minutes)

**What**: Reduce Cloud Run costs by 60-70%

```bash
# Deploy
cd terraform
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"

# Get URLs
BACKEND_URL=$(terraform output -raw backend_url)
CONSUMER_URL=$(terraform output -raw consumer_url)

# Verify
curl $BACKEND_URL/health

echo "✅ Phase 1 complete!"
echo "Backend: $BACKEND_URL"
echo "Savings: $4-7/month"
```

**Verify in console**:
1. Go to Cloud Run in GCP Console
2. Check `clicker-backend` and `clicker-consumer` memory settings
3. Should show "256Mi" and "Max instances: 10" / "5"

---

## Phase 2: Frontend Hosting (20 minutes)

**What**: Deploy frontend to Cloud CDN globally

```bash
cd ..

# Deploy frontend infrastructure
cd terraform
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"
cd ..

# Deploy frontend files
chmod +x scripts/deploy-frontend.sh
./scripts/deploy-frontend.sh

# Get URL
cd terraform
FRONTEND_URL=$(terraform output -raw frontend_url)
cd ..

# Verify
curl -I $FRONTEND_URL

echo "✅ Phase 2 complete!"
echo "Frontend: $FRONTEND_URL"
echo "Savings: Additional $3-5/month"
```

**Test in browser**:
1. Open `$FRONTEND_URL` in your browser
2. Click the button 10 times
3. Counter should increment
4. Leaderboard should update in real-time

---

## Phase 3: Cloud Functions (Optional, 30 minutes)

**What**: Replace consumer with zero-idle-cost Cloud Functions

**Only do this if you understand the tradeoffs!**

```bash
# Edit Terraform files
cd terraform

# 1. Uncomment Cloud Functions (cloudfunctions.tf)
vi cloudfunctions.tf
# Find the line: /* Commented out - uncomment to enable
# Remove the opening /* and closing */
# Save and quit (ESC, :wq)

# 2. Comment out Cloud Run consumer (cloudrun.tf)
vi cloudrun.tf
# Find: resource "google_cloud_run_service" "consumer" {
# Add /* before it
# Find the closing }
# Add */ after it
# Save and quit

# 3. Deploy
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"

# Wait for build (5-10 minutes)
echo "⏳ Cloud Functions building... (5-10 min)"

# Verify
gcloud functions describe process-click-event --region=us-central1 --gen2

echo "✅ Phase 3 complete!"
echo "Savings: Additional $1-2/month"
echo "Total: $0-2/month"
```

**Test**:
```bash
# Make some clicks
BACKEND_URL=$(terraform output -raw backend_url)
for i in {1..10}; do
  curl -X POST $BACKEND_URL/click
  sleep 1
done

# Check logs
gcloud functions logs read process-click-event --limit=20
```

---

## All Done! 🎉

### Summary

**Phase 1**: ✅ Reduced Cloud Run by 60-70%
**Phase 2**: ✅ Added global frontend CDN
**Phase 3**: ⚪ Optional, for advanced users

### Your New Costs
- **Before**: $9-12/month
- **After Phases 1-2**: $3-6/month
- **After Phase 3**: $2-5/month

### Total Savings
- **$4-10/month** (now within GCP's free tier!)

---

## Getting URLs Later

```bash
cd /home/carlos/Desktop/DevProjects/ClickerGCP/terraform
terraform output -raw backend_url
terraform output -raw frontend_url
```

---

## Troubleshooting

### "gcloud: command not found"
Install GCP CLI: https://cloud.google.com/sdk/docs/install

### "terraform: command not found"
Install Terraform: https://www.terraform.io/downloads.html

### "Permission denied" on deploy script
```bash
chmod +x scripts/deploy-frontend.sh
```

### "Terraform apply fails"
```bash
# Enable required APIs
gcloud services enable compute.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable storage-api.googleapis.com
gcloud services enable firestore.googleapis.com

# Retry
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"
```

### Frontend shows "Backend URL not configured"
```bash
# Rerun deployment
./scripts/deploy-frontend.sh
```

---

## Want More Details?

- **Full Implementation Guide**: `docs/FREE_TIER_OPTIMIZATION.md`
- **Deployment Checklist**: `docs/DEPLOYMENT_CHECKLIST.md`
- **Summary**: `IMPLEMENTATION_SUMMARY.md`

---

## Questions?

Each phase is **independently reversible**. If something goes wrong:

```bash
# Phase 1 rollback: Revert resource variables
terraform apply \
  -var="gcp_project_id=$GCP_PROJECT_ID" \
  -var="backend_memory=512Mi" \
  -var="consumer_memory=512Mi" \
  -var="backend_max_instances=100" \
  -var="consumer_max_instances=50"

# Phase 2 rollback: Delete frontend infrastructure
terraform destroy -target=google_storage_bucket.frontend \
  -var="gcp_project_id=$GCP_PROJECT_ID"

# Phase 3 rollback: Restore Cloud Run consumer
# Just comment out Cloud Functions and uncomment Cloud Run consumer
```

**Everything is reversible. Don't worry!**

---

## Time Breakdown

| Phase | Time | Risk | Recommended |
|-------|------|------|-------------|
| 1 | 15 min | Low | YES |
| 2 | 20 min | Low | YES |
| 3 | 30 min | Medium | Optional |

**Total for recommended setup**: ~35 minutes

---

Enjoy your cost savings! 🚀
