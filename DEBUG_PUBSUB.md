# Pub/Sub Message Flow Debugging Guide

## Quick Checklist - Run These Commands

### 1. Verify Pub/Sub Topic Exists
```bash
gcloud pubsub topics list --project=$GCP_PROJECT_ID

# Expected: click-events topic should be listed
```

### 2. Verify Subscription Exists and is Configured for Push
```bash
gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID

# Look for:
# - pushConfig.pushEndpoint: should be consumer's Cloud Run URL + /process
# - pushConfig.oidcToken.serviceAccountEmail: should be consumer's service account
```

### 3. Get the Consumer's Cloud Run Service URL
```bash
gcloud run services describe clicker-consumer --region=europe-southwest1 --project=$GCP_PROJECT_ID

# Look for: serviceConfig.uri (this is what push_endpoint should be)
```

### 4. Check Backend Can Publish (Service Account Permissions)
```bash
gcloud projects get-iam-policy $GCP_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.role:roles/pubsub.publisher" \
  --format="value(bindings.members)"

# Should include: serviceAccount:clicker-backend@$GCP_PROJECT_ID.iam.gserviceaccount.com
```

### 5. Check Backend Service is Actually Running
```bash
gcloud run services describe clicker-backend --region=europe-southwest1 --project=$GCP_PROJECT_ID

# Check: revisions exist and have status "ACTIVE"
```

### 6. Check Consumer Service is Actually Running
```bash
gcloud run services describe clicker-consumer --region=europe-southwest1 --project=$GCP_PROJECT_ID

# Check: revisions exist and have status "ACTIVE"
```

### 7. Test Backend Publishing Directly
```bash
# SSH into backend Cloud Run (or test locally) and check logs:
gcloud run services logs read clicker-backend --region=europe-southwest1 --project=$GCP_PROJECT_ID --limit=50

# Look for these log messages:
# ✓ Pub/Sub publisher initialized for topic 'click-events'
# OR
# ERROR: Failed to initialize Pub/Sub publisher:
```

### 8. Check Consumer /process Endpoint is Reachable
```bash
# Get consumer URL
CONSUMER_URL=$(gcloud run services describe clicker-consumer \
  --region=europe-southwest1 \
  --project=$GCP_PROJECT_ID \
  --format='value(status.url)')

# Test health endpoint (should work without auth)
curl -X GET $CONSUMER_URL/health

# Expected response: {"status":"ready",...} or {"status":"initializing",...}
```

### 9. Check Pub/Sub Messages in Topic (Dead Letter or Pending)
```bash
# Create a temporary pull subscription to see if messages exist
gcloud pubsub subscriptions create test-pull --topic=click-events --project=$GCP_PROJECT_ID

# Pull messages (don't ack them so they stay)
gcloud pubsub subscriptions pull test-pull --project=$GCP_PROJECT_ID --limit=5 --auto-ack=false

# Clean up
gcloud pubsub subscriptions delete test-pull --project=$GCP_PROJECT_ID
```

### 10. Check Pub/Sub Subscription Metrics
```bash
# View push subscription metrics (delivery attempts, etc.)
gcloud monitoring time-series list \
  --filter='metric.type="pubsub.googleapis.com/subscription/push_request_latencies"' \
  --project=$GCP_PROJECT_ID

# Or check Cloud Logging:
gcloud logging read "resource.type=cloud_pubsub_subscription" \
  --limit=20 \
  --project=$GCP_PROJECT_ID
```

---

## Common Issues & Solutions

### Issue 1: Topic Doesn't Exist
**Symptom:** `ERROR: Failed to initialize Pub/Sub publisher: topic click-events does not exist`

**Solution:**
```bash
# Create the topic
gcloud pubsub topics create click-events --project=$GCP_PROJECT_ID

# Verify
gcloud pubsub topics list --project=$GCP_PROJECT_ID
```

---

### Issue 2: Subscription Not Configured for Push
**Symptom:** Messages stuck in topic, consumer never called

**Solution:**
```bash
# Delete old subscription (if it exists)
gcloud pubsub subscriptions delete click-consumer-sub --project=$GCP_PROJECT_ID

# Get consumer URL
CONSUMER_URL=$(gcloud run services describe clicker-consumer \
  --region=europe-southwest1 --project=$GCP_PROJECT_ID --format='value(status.url)')

# Get consumer service account
CONSUMER_SA=$(gcloud iam service-accounts list \
  --project=$GCP_PROJECT_ID \
  --filter="displayName:clicker-consumer" \
  --format="value(email)")

# Create subscription with push
gcloud pubsub subscriptions create click-consumer-sub \
  --topic=click-events \
  --push-endpoint=$CONSUMER_URL/process \
  --push-auth-service-account=$CONSUMER_SA \
  --project=$GCP_PROJECT_ID
```

---

### Issue 3: Wrong Consumer URL in Subscription
**Symptom:** Push requests failing, 404 errors in logs

**Check & Fix:**
```bash
# Get current subscription config
gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID

# If pushConfig.pushEndpoint is wrong, re-create:
CONSUMER_URL=$(gcloud run services describe clicker-consumer \
  --region=europe-southwest1 --project=$GCP_PROJECT_ID --format='value(status.url)')

# Update subscription
gcloud pubsub subscriptions update click-consumer-sub \
  --push-endpoint=$CONSUMER_URL/process \
  --project=$GCP_PROJECT_ID
```

---

### Issue 4: Service Account Permissions Missing
**Symptom:** Backend logs: `Error publishing: permission denied`

**Solution:**
```bash
# Get backend service account
BACKEND_SA=$(gcloud iam service-accounts list \
  --project=$GCP_PROJECT_ID \
  --filter="displayName:clicker-backend" \
  --format="value(email)")

# Grant pubsub.publisher role
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member=serviceAccount:$BACKEND_SA \
  --role=roles/pubsub.publisher

# Get consumer service account
CONSUMER_SA=$(gcloud iam service-accounts list \
  --project=$GCP_PROJECT_ID \
  --filter="displayName:clicker-consumer" \
  --format="value(email)")

# Grant pubsub.subscriber role
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member=serviceAccount:$CONSUMER_SA \
  --role=roles/pubsub.subscriber
```

---

### Issue 5: Backend Not Calling PublishClickEvent
**Symptom:** Logs show clicks received, but no publish logs

**Check in Backend Logs:**
```bash
# Look for publish errors or missing publisher
gcloud run services logs read clicker-backend \
  --region=europe-southwest1 \
  --project=$GCP_PROJECT_ID \
  --limit=50 | grep -i "publish\|pubsub"
```

**In code, verify `/click` handler calls publisher:**
```go
// In /click handler after geolocating IP:
if publisher != nil {
    err := publisher.PublishClickEvent(bgCtx, country, clientIP)
    if err != nil {
        log.Printf("Failed to publish click event: %v", err)
    } else {
        log.Printf("✓ Published click event: country=%s, ip=%s", country, clientIP)
    }
}
```

---

### Issue 6: Consumer Service Returns Errors
**Symptom:** Pub/Sub tries to push, consumer returns 400/500

**Solution:**
```bash
# Check consumer logs
gcloud run services logs read clicker-consumer \
  --region=europe-southwest1 \
  --project=$GCP_PROJECT_ID \
  --limit=100

# Look for patterns like:
# [/process] ERROR: ...
# [/process] ✓ SUCCESS ...
```

**Test /process endpoint manually:**
```bash
CONSUMER_URL=$(gcloud run services describe clicker-consumer \
  --region=europe-southwest1 --project=$GCP_PROJECT_ID --format='value(status.url)')

# Create a test message (same format as backend publishes)
TEST_PAYLOAD=$(cat <<'EOF'
{
  "message": {
    "messageId": "test-123",
    "data": "eyJ0aW1lc3RhbXAiOjE3Mzg1MDIyNDUsImNvdW50cnkiOiJVUyIsImlwIjoiMTkyLjE2OC4xLjEifQ=="
  }
}
EOF
)

# Send it
curl -X POST $CONSUMER_URL/process \
  -H "Content-Type: application/json" \
  -d "$TEST_PAYLOAD" \
  -v
```

---

## Monitoring & Observability

### View All Pub/Sub Activity
```bash
gcloud logging read "resource.type=pubsub_topic OR resource.type=pubsub_subscription" \
  --limit=50 \
  --format=json \
  --project=$GCP_PROJECT_ID | jq '.[] | {timestamp: .timestamp, message: .textPayload, severity: .severity}'
```

### Check Subscription Dead Letter Topic (if enabled)
```bash
gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID | grep -i deadletter
```

### Monitor in Real Time
```bash
# Watch logs as they happen
gcloud logging tail "resource.type=cloud_run_revision" \
  --limit=50 \
  --follow \
  --project=$GCP_PROJECT_ID
```

---

## Step-by-Step Diagnosis Flow

1. **Does the topic exist?**
   ```bash
   gcloud pubsub topics describe click-events --project=$GCP_PROJECT_ID
   ```
   - NO → Create it (see Issue 1)
   - YES → Continue

2. **Does the subscription exist?**
   ```bash
   gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID
   ```
   - NO → Create it (see Issue 2)
   - YES → Continue

3. **Is the subscription push config correct?**
   ```bash
   gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID \
     --format='value(pushConfig.pushEndpoint)'
   ```
   - Ends with `/process`? YES → Continue
   - NO → Fix it (see Issue 3)

4. **Is backend service running?**
   ```bash
   gcloud run services describe clicker-backend --region=europe-southwest1 --project=$GCP_PROJECT_ID
   ```
   - YES → Continue
   - NO → Deploy it

5. **Does backend have pubsub.publisher permission?**
   ```bash
   gcloud projects get-iam-policy $GCP_PROJECT_ID --flatten="bindings[].members" \
     --filter="bindings.role:roles/pubsub.publisher"
   ```
   - Includes backend service account? YES → Continue
   - NO → Grant it (see Issue 4)

6. **Is backend publishing?**
   ```bash
   gcloud run services logs read clicker-backend --limit=50 | grep -i publish
   ```
   - See "✓ Pub/Sub publisher initialized"? YES → Continue
   - See "ERROR"? → Check credentials/permissions
   - See "Continuing without Pub/Sub"? → Topic doesn't exist (see Issue 1)

7. **Are there messages in the topic?**
   ```bash
   gcloud pubsub subscriptions create test-pull --topic=click-events
   gcloud pubsub subscriptions pull test-pull --limit=5 --auto-ack=false
   gcloud pubsub subscriptions delete test-pull
   ```
   - See messages? YES → Problem is in push delivery
   - NO → Problem is in publishing (steps 1-6 failed)

8. **Is consumer service running?**
   ```bash
   gcloud run services describe clicker-consumer --region=europe-southwest1 --project=$GCP_PROJECT_ID
   ```
   - YES → Continue
   - NO → Deploy it

9. **Does consumer have pubsub.subscriber permission?**
   ```bash
   gcloud projects get-iam-policy $GCP_PROJECT_ID --flatten="bindings[].members" \
     --filter="bindings.role:roles/pubsub.subscriber"
   ```
   - Includes consumer service account? YES → Continue
   - NO → Grant it (see Issue 4)

10. **Can consumer process messages?**
    ```bash
    curl -X GET $CONSUMER_URL/health
    # Should return: {"status":"ready",...}

    # Try posting a test message (see Issue 6)
    ```
    - Returns 200 with {"status":"ready"}? YES → Everything OK
    - Returns 500? Check logs → Fix errors

---

## Expected Flow When Working

1. Frontend clicks → calls `/click` endpoint
2. Backend receives click, geolocates IP
3. Backend publishes event to Pub/Sub topic `click-events`
4. Pub/Sub subscription `click-consumer-sub` receives message
5. Pub/Sub pushes to consumer's `$CONSUMER_URL/process` endpoint (HTTP POST)
6. Consumer decodes base64, unmarshals JSON, increments Firestore counters
7. Consumer calls backend's `/internal/broadcast` to update connected WebSocket clients
8. Frontend sees counter update in real-time

---

## Questions to Answer

After running the above diagnostics, answer these:

1. Does the topic `click-events` exist in your GCP project?
2. Does the subscription `click-consumer-sub` exist?
3. What is the `pushEndpoint` URL in the subscription config?
4. What is your consumer Cloud Run service URL?
5. Do they match? (pushEndpoint should be: `{consumer_url}/process`)
6. When you check backend logs, do you see "✓ Pub/Sub publisher initialized" or an error?
7. When you send a test click to the backend's `/click` endpoint, do you see a publish log?
8. When you run the temporary pull subscription, do you see any messages in the topic?

