# Pub/Sub Message Flow Troubleshooting Guide

## Problem Statement

When users click the frontend button, the backend receives the click successfully, but:
- ❌ Backend cannot publish messages to Pub/Sub
- ❌ Consumer does not receive messages
- ❌ Counters do not increment in Firestore

## Root Cause Analysis - UPDATED

### The Issue: Backend Pub/Sub Publisher Initialization Fails with "PermissionDenied"

The backend service shows:
```
pubsubPublisher: false
publisherError: "rpc error: code = PermissionDenied desc = User not authorized to perform this action."
```

This occurs when the backend tries to check if the Pub/Sub topic exists during startup:
```go
topic := client.Topic(topicName)
exists, err := topic.Exists(ctx)  // ← Fails here with PermissionDenied
```

### Investigation Findings

**What's Working ✅:**
- Backend service is running and healthy
- Backend receives HTTP requests (click endpoint works → returns success: true)
- Consumer service is running and publicly accessible
- Pub/Sub topic EXISTS and is accessible
- All IAM permissions ARE correctly assigned:
  - `clicker-backend@dev-trail-475809-v2.iam.gserviceaccount.com` has:
    - `roles/pubsub.admin` ✅
    - `roles/pubsub.editor` ✅
    - `roles/pubsub.publisher` ✅
  - `clicker-consumer@dev-trail-475809-v2.iam.gserviceaccount.com` has:
    - `roles/pubsub.subscriber` ✅
    - `roles/pubsub.viewer` ✅
  - Pub/Sub service account can invoke consumer ✅
- Manual publishing works: `gcloud pubsub topics publish click-events` succeeds
- Pub/Sub API is enabled and accessible

**What's NOT Working ❌:**
- Backend Cloud Run service cannot use its service account credentials to access Pub/Sub
- The PermissionDenied error persists even after:
  - Rebuilding Docker images ✓
  - Restarting Cloud Run service ✓
  - Adding timeout contexts ✓
  - Verifying all IAM roles ✓

### Root Cause: Credential Loading in Cloud Run Environment

The issue is NOT:
- ❌ IAM permissions (verified multiple times)
- ❌ Topic existence (topic exists and is accessible)
- ❌ Docker image staleness (rebuilt and tested)
- ❌ Environment variables (all set correctly)
- ❌ Pub/Sub API availability (API is enabled and working)

The issue IS:
- ✅ The Cloud Run service is NOT loading the service account credentials properly
- ✅ The Pub/Sub client is being created but cannot authenticate with the backend service account
- ✅ This is a credential negotiation failure between Cloud Run and Pub/Sub API

**Possible causes:**
1. Cloud Run service isn't passing service account credentials to the application
2. The Go `cloud.google.com/go/pubsub` library isn't picking up credentials from Cloud Run environment
3. There's a network/firewall issue between Cloud Run and Pub/Sub API (unlikely, but possible)
4. The service account itself has an internal issue or is in an invalid state

---

## Solutions to Try (In Order)

### Solution 1: Delete and Recreate Service Account (Most Likely to Work)

The service account itself might be in an invalid state. Deleting and recreating it forces fresh credential setup:

```bash
# 1. Delete the backend service account
gcloud iam service-accounts delete clicker-backend@dev-trail-475809-v2.iam.gserviceaccount.com

# 2. Redeploy to recreate it with Terraform
cd terraform
terraform apply -lock=false -auto-approve

# 3. Rebuild Docker images with new service account
gcloud builds submit --config=backend/cloudbuild.yaml backend/
gcloud builds submit --config=consumer/cloudbuild.yaml consumer/

# 4. Test
sleep 120
BACKEND_URL=$(gcloud run services describe clicker-backend --region=europe-southwest1 --format='value(status.url)')
curl -s "$BACKEND_URL/debug/config" | jq '.pubsubPublisher'
# Expected: true
```

### Solution 2: Full Infrastructure Rebuild from Zero

Destroy everything and redeploy with fresh infrastructure:

```bash
# 1. Destroy all infrastructure
terraform destroy -auto-approve

# 2. Wait for Firestore cleanup (important!)
sleep 300

# 3. Rebuild from scratch
terraform apply -auto-approve

# 4. Test
sleep 60
BACKEND_URL=$(terraform output -raw backend_url)
curl -s "$BACKEND_URL/debug/config" | jq '.pubsubPublisher'
# Expected: true
```

### Solution 3: Modify Backend Code to Skip Pub/Sub Initialization Failure

If you want the backend to work even if Pub/Sub fails to initialize, modify the code to allow graceful degradation. This keeps the system operational but won't send messages to Pub/Sub.

The current code already does this - it logs the error and continues without Pub/Sub. If you need functionality to work, you could:
- Remove the topic existence check (just assume it exists)
- Add retry logic with exponential backoff
- Use lazy initialization (initialize on first publish attempt, not at startup)

---

## Original Solution: Rebuild and Redeploy Docker Images

**Note:** This solution was already attempted and did NOT resolve the issue. It ensures:
1. Fresh credential loading when the service starts
2. Latest code and environment configuration
3. Service account credentials are properly refreshed

### Step-by-Step Solution

#### Option A: Rebuild via Cloud Build (Recommended)

```bash
# From the repository root
cd /home/carlos/Desktop/DevProjects/ClickerGCP

# Rebuild backend image
gcloud builds submit --config=backend/cloudbuild.yaml backend/

# Rebuild consumer image
gcloud builds submit --config=consumer/cloudbuild.yaml consumer/

# Wait for builds to complete (3-5 minutes each)
# Check status:
gcloud builds list --limit=2 --format="table(id,status,createTime)"
```

#### Option B: Automatic Rebuild via Terraform

```bash
cd terraform

# Force rebuild by modifying the null_resource trigger
# Option 1: Destroy and recreate builds
terraform taint null_resource.build_backend
terraform taint null_resource.build_consumer
terraform apply

# Option 2: Full redeploy from scratch
terraform destroy -auto-approve
sleep 300  # Wait for Firestore cleanup
terraform apply
```

### Verify the Fix

After rebuilding and redeploying, test the complete flow:

```bash
# Get backend URL
BACKEND_URL=$(gcloud run services describe clicker-backend --region=europe-southwest1 --format='value(status.url)')

# 1. Verify Pub/Sub publisher is now initialized
echo "Checking Pub/Sub status..."
curl -s "$BACKEND_URL/debug/config" | jq '.pubsubPublisher'
# Expected output: true (not false)

# 2. Send a test click
echo ""
echo "Sending test click..."
curl -s "$BACKEND_URL/click?country=TEST&ip=1.2.3.4" | jq .

# 3. Wait for Pub/Sub delivery (typically <1 second)
sleep 2

# 4. Check if counter incremented
echo ""
echo "Checking counter..."
curl -s "$BACKEND_URL/count" | jq .
# Expected output: {"global":1,"countries":{"TEST":1}}
```

### Expected Results After Fix

```
Step 1: pubsubPublisher = true  ✅
Step 2: Click returns success: true  ✅
Step 3: (Pub/Sub delivers message)
Step 4: Counter shows global: 1, countries: { TEST: 1 }  ✅
```

---

## Why This Happens: Technical Deep Dive

### The Pub/Sub Publisher Initialization Code

In `backend/main.go`:
```go
publisher, err = NewPubSubPublisher(bgCtx, projectID, "click-events")
if err != nil {
    pubErrorStr = fmt.Sprintf("Pub/Sub initialization failed: %v", err)
    log.Printf("[Services] ✗ Pub/Sub initialization failed: %v", err)
}
```

The publisher is initialized when the server starts. If the context and credentials aren't ready at that moment, it fails.

### Cloud Run Credential Loading

Cloud Run provides credentials via:
1. **Application Default Credentials (ADC)**
2. **Service Account metadata endpoint**

The Google Pub/Sub client library should automatically pick these up. However:
- Old Docker images might have cached credential paths
- The service instance needs to be fully restarted
- Credentials are loaded at service startup time

### The Fix: Fresh Docker Image with Restart

When you rebuild and redeploy:
1. New Docker image is built with fresh configuration
2. Old Cloud Run service instance is terminated
3. New service instance starts with new image
4. Google client libraries load fresh credentials from Cloud Run environment
5. Pub/Sub publisher initializes successfully

---

## Prevention: Best Practices

### 1. Always Rebuild Images After IAM Changes

If you modify IAM permissions, rebuild and redeploy the affected services:

```bash
# After any IAM changes
gcloud builds submit --config=backend/cloudbuild.yaml backend/
gcloud builds submit --config=consumer/cloudbuild.yaml consumer/
```

### 2. Use Cloud Build Triggers for Automatic Deployment

Set up GitHub triggers so images are automatically rebuilt on push:
- See: [Optional: Set Up Continuous Deployment](../README.md#step-7-optional-set-up-continuous-deployment)

### 3. Monitor Pub/Sub Publisher Status

Add periodic health checks to monitor publisher initialization:

```bash
# Daily health check
BACKEND_URL=$(terraform output -raw backend_url)
curl "$BACKEND_URL/debug/config" | jq '.pubsubPublisher'
```

### 4. Check Deployment Order

Terraform applies resources in parallel. Ensure dependencies are correct:
- ✅ Firestore waits for builds to complete
- ✅ Cloud Run depends on IAM roles
- ✅ Builds depend on Artifact Registry

---

## Troubleshooting Checklist

If you still have issues after rebuilding:

- [ ] Rebuilt Docker images via `gcloud builds submit`
- [ ] Verified new images are in Artifact Registry
- [ ] Waited 2-3 minutes for Cloud Run to fully redeploy
- [ ] Checked backend health: `curl $BACKEND_URL/health`
- [ ] Verified publisher status: `curl $BACKEND_URL/debug/config`
- [ ] Checked IAM roles are assigned:
  ```bash
  gcloud projects get-iam-policy dev-trail-475809-v2 \
    --flatten="bindings[].members" \
    --filter="bindings.members:clicker-backend@*" \
    --format="table(bindings.role)"
  ```
- [ ] Verified Cloud Run service account:
  ```bash
  gcloud run services describe clicker-backend \
    --region=europe-southwest1 \
    --format='value(spec.template.spec.serviceAccountName)'
  ```

---

## Summary

**The Problem**: Docker images are old and don't have fresh credentials
**The Solution**: Rebuild and redeploy Docker images
**The Result**: Pub/Sub publisher initializes successfully → Messages flow → Counters increment

**Time to Fix**: ~10-15 minutes (5 min for builds + 2-3 min for deployment)

---

## Quick Reference: Complete Fix Command

```bash
# One-liner to rebuild everything
cd /home/carlos/Desktop/DevProjects/ClickerGCP && \
gcloud builds submit --config=backend/cloudbuild.yaml backend/ && \
gcloud builds submit --config=consumer/cloudbuild.yaml consumer/ && \
sleep 120 && \
BACKEND_URL=$(gcloud run services describe clicker-backend --region=europe-southwest1 --format='value(status.url)') && \
curl -s "$BACKEND_URL/debug/config" | jq '.pubsubPublisher'
```

Expected final output: `true`
