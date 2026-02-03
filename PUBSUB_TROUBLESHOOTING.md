# Pub/Sub Message Flow Troubleshooting Guide

## Problem Statement

When users click the frontend button, the backend receives the click successfully, but:
- ❌ Backend cannot publish messages to Pub/Sub
- ❌ Consumer does not receive messages
- ❌ Counters do not increment in Firestore

## Root Cause Analysis

### The Issue: Backend Pub/Sub Publisher Initialization Fails

The backend service logs show:
```
PermissionDenied: User not authorized to perform this action.
```

This occurs when the backend tries to check if the Pub/Sub topic exists during startup:
```go
topic := client.Topic(topicName)
exists, err := topic.Exists(ctx)  // ← Fails here
```

### Why It Happens

1. **Docker Image Out of Date**: The Docker images in Artifact Registry were built BEFORE IAM permissions were properly configured
2. **Stale Credentials**: The running Cloud Run service is using a Docker image that might have cached or outdated credential information
3. **Credential Loading Issue**: The application initializes Pub/Sub on startup, and if credentials aren't properly available at that moment, the initialization fails
4. **IAM Propagation**: Even though all IAM roles are set correctly, the Cloud Run service instance needs to be restarted to pick up new credentials

### What's Working ✅

- Backend service is running and healthy
- Backend receives HTTP requests (click endpoint works)
- Consumer service is running and publicly accessible
- Pub/Sub topic and subscription are properly configured
- All IAM permissions are correctly assigned:
  - Backend has `roles/pubsub.publisher`
  - Consumer has `roles/pubsub.subscriber`
  - Pub/Sub service account can invoke consumer

### What's NOT Working ❌

- Backend cannot initialize Pub/Sub publisher at startup
- No messages are being published to Pub/Sub
- Consumer never receives messages to process
- Counters never increment

---

## Solution: Rebuild and Redeploy Docker Images

The fix is to rebuild the Docker images and redeploy them to Cloud Run. This ensures:
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
