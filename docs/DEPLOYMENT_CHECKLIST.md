# GCP Free Tier Optimization - Deployment Checklist

## Quick Start

```bash
export GCP_PROJECT_ID="your-project-id"
cd /home/carlos/Desktop/DevProjects/ClickerGCP
```

---

## Phase 1: Cloud Run Optimization

### Summary
Reduce Cloud Run costs by 60-70% with optimized resource allocation.
- Memory: 512Mi → 256Mi
- Max Instances: 100 → 10 (backend), 50 → 5 (consumer)
- Timeout: 300s → 60s

### Deployment

```bash
cd terraform

# Review changes
terraform plan -var="gcp_project_id=$GCP_PROJECT_ID"

# Apply changes
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"

# Verify
BACKEND_URL=$(terraform output -raw backend_url)
curl $BACKEND_URL/health

# Check configuration
gcloud run services describe clicker-backend \
  --region=us-central1 \
  --format="table(spec.template.spec.containers[0].resources)"
```

**Expected Result**: Memory shows "256Mi", maxScale shows "10"

**Rollback** (if needed):
```bash
terraform apply \
  -var="gcp_project_id=$GCP_PROJECT_ID" \
  -var="backend_memory=512Mi" \
  -var="consumer_memory=512Mi" \
  -var="backend_max_instances=100" \
  -var="consumer_max_instances=50" \
  -var="request_timeout=300"
```

---

## Phase 2: Frontend Static Hosting

### Summary
Deploy frontend to Cloud Storage with global CDN.
- Static files cached globally
- Reduces bandwidth costs
- Improves load times

### Deployment

```bash
cd terraform

# Review and apply changes
terraform plan -var="gcp_project_id=$GCP_PROJECT_ID"
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"

cd ..

# Make script executable
chmod +x scripts/deploy-frontend.sh

# Deploy frontend
./scripts/deploy-frontend.sh

# Get frontend URL
cd terraform
FRONTEND_URL=$(terraform output -raw frontend_url)
cd ..

# Verify
echo "Frontend URL: $FRONTEND_URL"
curl -I $FRONTEND_URL
```

**Expected Result**:
- HTTP 200 response
- Cache-Control headers present
- Content-Length shows HTML size (~5-10KB)

**Verify WebSocket Connection**:
```bash
# Open frontend in browser at $FRONTEND_URL
# Check browser console:
# - Should show "WebSocket Connected"
# - Click button should increment counter
# - Leaderboard should update in real-time
```

**Rollback** (if needed):
```bash
cd terraform

# Delete frontend infrastructure
terraform destroy \
  -target=google_compute_global_forwarding_rule.frontend \
  -target=google_compute_global_address.frontend \
  -target=google_compute_target_http_proxy.frontend \
  -target=google_compute_url_map.frontend \
  -target=google_compute_backend_bucket.frontend \
  -target=google_storage_bucket.frontend \
  -var="gcp_project_id=$GCP_PROJECT_ID"

cd ..

# Frontend still works locally from frontend/ directory
```

---

## Phase 3: Cloud Functions Consumer (OPTIONAL)

### Summary
Replace Cloud Run consumer with event-driven Cloud Functions.
- No idle costs
- Cold starts: 2-4 seconds
- Only for sporadic workloads (<10K clicks/month)

### Prerequisites
- Phase 1 and 2 completed
- Comfortable with event-driven architecture
- Willing to test thoroughly

### Deployment

**Step 1: Prepare files**
```bash
# Cloud Functions code already in:
# - functions/consumer/main.go
# - functions/consumer/go.mod

ls -la functions/consumer/
```

**Step 2: Enable Cloud Functions**
```bash
# Edit terraform/cloudfunctions.tf
# Find the large comment block starting with: /* Commented out - uncomment to enable
# Remove the leading /* and trailing */

cd terraform
vi cloudfunctions.tf

# Should look like:
# resource "google_project_service" "cloudfunctions" {
#   service = "cloudfunctions.googleapis.com"
#   ...
```

**Step 3: Disable Cloud Run Consumer**
```bash
# Edit terraform/cloudrun.tf
# Comment out the entire consumer service block (lines 60-122)

vi cloudrun.tf

# Should look like:
# /* Temporarily disabled - using Cloud Functions instead
# resource "google_cloud_run_service" "consumer" {
#   ...
# }
# */
```

**Step 4: Deploy**
```bash
# Review changes
terraform plan -var="gcp_project_id=$GCP_PROJECT_ID"

# Apply (this will take a few minutes to build the function)
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"

# Wait for build to complete
echo "Waiting for Cloud Functions to build..."
sleep 60

# Check deployment status
gcloud functions describe process-click-event \
  --region=us-central1 \
  --gen2
```

**Step 5: Test**
```bash
# Make some clicks
BACKEND_URL=$(terraform output -raw backend_url)
for i in {1..10}; do
  curl -X POST $BACKEND_URL/click
  sleep 1
done

# Check logs
gcloud functions logs read process-click-event \
  --region=us-central1 \
  --limit=50

# Verify Firestore updates
gcloud firestore documents list counters

# Check frontend
FRONTEND_URL=$(terraform output -raw frontend_url)
curl -s $FRONTEND_URL | grep -o '"clicks":[0-9]*'
```

**Expected Result**:
- Logs show successful invocations
- Counter in Firestore increments
- WebSocket broadcasts reach browser

**Rollback** (if needed):
```bash
# Revert cloudfunctions.tf
cd terraform
git checkout cloudfunctions.tf

# Uncomment consumer in cloudrun.tf
vi cloudrun.tf

# Redeploy
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"
```

---

## Final Verification

### All Phases Deployed

```bash
# 1. Check Cloud Run services
echo "=== Cloud Run Services ==="
gcloud run services list --region=us-central1

# 2. Check Cloud Storage bucket
echo "=== Storage Bucket ==="
gsutil ls gs://${GCP_PROJECT_ID}-clicker-frontend/

# 3. Check Cloud CDN
echo "=== Cloud CDN Backend ==="
gcloud compute backend-buckets describe clicker-frontend-backend

# 4. Check Cloud Functions (if Phase 3 deployed)
echo "=== Cloud Functions ==="
gcloud functions list --gen2 2>/dev/null || echo "Cloud Functions not enabled"

# 5. Test backend health
echo "=== Backend Health ==="
BACKEND_URL=$(cd terraform && terraform output -raw backend_url)
curl -s $BACKEND_URL/health | jq .

# 6. Test frontend
echo "=== Frontend ==="
FRONTEND_URL=$(cd terraform && terraform output -raw frontend_url)
curl -s $FRONTEND_URL | head -20

# 7. Test click flow
echo "=== Testing Click Flow ==="
curl -X POST $BACKEND_URL/click \
  -H "X-Forwarded-For: 192.0.2.1" \
  -w "\nHTTP Status: %{http_code}\n"

# 8. Verify Firestore
echo "=== Firestore Counters ==="
gcloud firestore documents list counters
```

---

## Cost Verification

### Before Optimization
```
Backend (Cloud Run):   $7-9/month
Consumer (Cloud Run):  $2-3/month
Frontend:              $0
---
Total:                 $9-12/month
Status:                ❌ Over free tier
```

### After Phase 1
```
Backend (Cloud Run):   $2-3/month
Consumer (Cloud Run):  $1-2/month
Frontend:              $0
---
Total:                 $3-5/month
Status:                ✅ Within free tier
Savings:               $4-7/month
```

### After Phase 2
```
Backend (Cloud Run):   $2-3/month
Consumer (Cloud Run):  $1-2/month
Frontend (CDN):        $0-1/month
---
Total:                 $3-6/month
Status:                ✅ Within free tier
Savings:               $3-9/month
```

### After Phase 3 (Optional)
```
Backend (Cloud Run):        $2-3/month
Consumer (Cloud Functions): $0-1/month
Frontend (CDN):             $0-1/month
---
Total:                      $2-5/month
Status:                      ✅ Within free tier
Savings:                     $4-10/month
```

**To verify**: Check GCP Billing Dashboard → Cost Management → Forecast

---

## Monitoring

### Key Commands

```bash
# Real-time logs
gcloud logging read "resource.type=cloud_run_revision" --limit=20 --follow

# Error rates
gcloud logging read "severity>=ERROR" --limit=50

# Firestore operations
gcloud logging read "resource.type=firestore_database" --limit=50

# Cloud Functions
gcloud logging read "resource.type=cloud_function" --limit=50

# View metrics
gcloud monitoring metrics list --filter='metric.type:run.googleapis.com' | head -20
```

### Performance Baselines

| Metric | Phase 1 | Phase 2 | Phase 3 |
|--------|---------|---------|---------|
| Backend response time (p50) | <100ms | <100ms | <100ms |
| Backend response time (p95) | <500ms | <500ms | <500ms |
| Frontend load time | N/A | ~0.5s global | ~0.5s global |
| Cold start (first request) | <1s | <1s | 2-4s |
| Warm request latency | <100ms | <100ms | <100ms |

---

## Troubleshooting

### Issue: Terraform apply fails

```bash
# Check if APIs are enabled
gcloud services list --enabled | grep -E "compute|run|storage|firestore"

# Manually enable if needed
gcloud services enable compute.googleapis.com
gcloud services enable run.googleapis.com
gcloud services enable storage-api.googleapis.com
gcloud services enable firestore.googleapis.com

# Retry apply
terraform apply -var="gcp_project_id=$GCP_PROJECT_ID"
```

### Issue: Frontend shows "Backend URL not configured"

```bash
# Verify meta tag was set
grep "backend-url" frontend/index.html

# Check Terraform output
cd terraform
terraform output frontend_url
terraform output backend_url
cd ..

# Rerun deployment script
./scripts/deploy-frontend.sh
```

### Issue: WebSocket connection fails

```bash
# Check backend logs
BACKEND_URL=$(cd terraform && terraform output -raw backend_url)
gcloud logging read "resource.labels.service_name=clicker-backend" --limit=50

# Verify CORS is enabled
curl -I $BACKEND_URL/ws

# Check security groups/firewall
gcloud compute firewall-rules list | grep -i clicker
```

### Issue: Cloud Functions not triggering

```bash
# Verify Pub/Sub subscription
gcloud pubsub subscriptions describe click-consumer-sub

# Check subscription push endpoint
gcloud pubsub subscriptions describe click-consumer-sub \
  --format="value(pushConfig.pushEndpoint)"

# Check Cloud Functions logs for errors
gcloud functions logs read process-click-event --limit=50

# Manually test Pub/Sub message
gcloud pubsub topics publish click-events \
  --message='{"timestamp":"2024-01-01T00:00:00Z","country":"US","ip":"192.0.2.1"}'
```

---

## Next Steps

1. **Monitor costs** for 1-2 weeks to confirm savings
2. **Review logs** for any performance degradation
3. **Test load capacity** with load testing tool (e.g., `hey` or `wrk`)
4. **Consider Phase 3** if traffic pattern suits event-driven model
5. **Document** any custom configurations for future reference

---

## Timeline

| Phase | Risk | Effort | Savings | Timeline |
|-------|------|--------|---------|----------|
| 1 | Low | 30 min | $4-5/mo | Immediate |
| 2 | Low | 45 min | $3-5/mo | After Phase 1 |
| 3 | Medium | 1 hour | $1-2/mo | Optional |

**Total**: 2-2.5 hours for Phases 1-2 (recommended)

---

## Support

For detailed information, see:
- `docs/FREE_TIER_OPTIMIZATION.md` - Complete implementation guide
- `docs/ARCHITECTURE.md` - System design
- `docs/SETUP.md` - Initial setup guide
- `README.md` - Project overview

For issues, check the troubleshooting section above or review GCP Console logs.
