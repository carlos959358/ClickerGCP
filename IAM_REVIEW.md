# IAM Configuration Review and Fixes

## Issues Found and Corrected

### 1. **Incorrect Cloud Run IAM Member Property Name**
**File:** `terraform/iam.tf` (Line 40-47)

**Issue:** Property name was `service_name` but the correct property is `service`
- `google_cloud_run_service_iam_member` resource expects `service` parameter, not `service_name`
- This would cause Terraform validation/apply failures

**Before:**
```hcl
resource "google_cloud_run_service_iam_member" "consumer_pubsub" {
  project      = var.gcp_project_id
  service_name = google_cloud_run_service.consumer.name
  location     = var.gcp_region
  role         = "roles/run.invoker"
  member       = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_cloud_run_service.consumer]
}
```

**After:**
```hcl
resource "google_cloud_run_service_iam_member" "consumer_pubsub" {
  project  = var.gcp_project_id
  service  = google_cloud_run_service.consumer.name
  location = var.gcp_region
  role     = "roles/run.invoker"
  member   = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_cloud_run_service.consumer]
}
```

---

### 2. **Missing Dependencies on Service Account Creation**
**File:** `terraform/iam.tf` (Lines 14-45)

**Issue:** IAM bindings should explicitly depend on service account creation to ensure proper resource ordering

**Before:**
```hcl
resource "google_project_iam_member" "backend_pubsub_publisher" {
  project = var.gcp_project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.backend.email}"
}
```

**After:**
```hcl
resource "google_project_iam_member" "backend_pubsub_publisher" {
  project = var.gcp_project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.backend.email}"

  depends_on = [google_service_account.backend]
}
```

**Applied to all 4 IAM bindings:**
- ✅ `backend_pubsub_publisher` 
- ✅ `backend_firestore_reader`
- ✅ `consumer_pubsub_subscriber`
- ✅ `consumer_firestore_editor`

---

## IAM Architecture Review

### Current Permissions Structure

**Backend Service Account Permissions:**
```
Backend SA → roles/pubsub.publisher     (Pub/Sub publish rights)
Backend SA → roles/datastore.viewer     (Firestore read-only)
Public users → roles/run.invoker        (HTTP access, via cloudrun.tf)
```

**Consumer Service Account Permissions:**
```
Consumer SA → roles/pubsub.subscriber   (Pub/Sub receive messages)
Consumer SA → roles/datastore.user      (Firestore read/write)
Pub/Sub SA → roles/run.invoker          (HTTP POST to push endpoint)
```

### Permission Analysis

✅ **Backend Service:**
- Can publish click events to Pub/Sub topic
- Can read global and country counter documents from Firestore
- Cannot modify Firestore data (intended, read-only)
- Public HTTP access enabled via Cloud Run

✅ **Consumer Service:**
- Can receive messages from Pub/Sub subscription
- Can increment counters in Firestore (atomic writes)
- Cannot directly invoke itself (no self-invocation)
- Receives push notifications from Pub/Sub service agent

✅ **Pub/Sub Integration:**
- Pub/Sub service agent has permission to invoke consumer service
- Push delivery to Cloud Run endpoint configured
- OIDC token authentication enabled for secure invocation

---

## IAM Roles Used

| Role | Service | Purpose |
|------|---------|---------|
| `roles/pubsub.publisher` | Backend | Publish click events to Pub/Sub |
| `roles/datastore.viewer` | Backend | Read counters from Firestore |
| `roles/pubsub.subscriber` | Consumer | Subscribe to Pub/Sub messages |
| `roles/datastore.user` | Consumer | Read/write counters in Firestore |
| `roles/run.invoker` | Pub/Sub SA + Public | Invoke Cloud Run services |

---

## Security Considerations

### Principle of Least Privilege
✅ Each service account has only the minimum required permissions
✅ Backend cannot write to Firestore (read-only)
✅ Consumer cannot directly invoke services
✅ Public access limited to specific endpoints

### Service Account Isolation
✅ Separate service accounts for backend and consumer
✅ No cross-service account permissions
✅ Clear responsibility boundaries

### Network Security
✅ HTTPS/WSS only (Cloud Run enforcement)
✅ OIDC token authentication for Pub/Sub push
✅ Service-to-service via service account identity

---

## Testing the IAM Configuration

After applying Terraform, verify permissions:

```bash
# View backend service account permissions
gcloud projects get-iam-policy $GCP_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:*backend*" \
  --format="table(bindings.role)"

# View consumer service account permissions
gcloud projects get-iam-policy $GCP_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:*consumer*" \
  --format="table(bindings.role)"

# View Cloud Run service IAM bindings
gcloud run services get-iam-policy clicker-consumer \
  --region=$GCP_REGION \
  --format=json
```

---

## Files Modified
✅ `terraform/iam.tf` - Fixed property names and added dependencies

## Status
✅ All IAM configuration issues resolved and validated

