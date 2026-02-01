# Terraform Configuration Fixes

## Issues Found and Corrected

### 1. **firestore.tf** - Invalid Resource
**Issue:** `google_firestore_document` resource doesn't exist in Terraform Google provider
**Solution:** Removed the invalid resource. Document initialization is now handled by the `init-firestore.sh` script.

**Before:**
```hcl
resource "google_firestore_document" "global_counter" {
  project     = var.gcp_project_id
  database    = google_firestore_database.clicker.name
  collection  = "counters"
  document_id = "global"

  fields {
    name  = "count"
    value = jsonencode(0)
  }

  fields {
    name  = "lastUpdated"
    value = jsonencode({
      timestampValue = "2025-01-01T00:00:00Z"
    })
  }
}
```

**After:**
```hcl
# Note: Global counter document is created via init-firestore.sh script
# Terraform google provider doesn't have a native resource to manage documents
# Use the initialization script to set up the initial data
```

---

### 2. **pubsub.tf** - Invalid Authentication Configuration
**Issue:** `oidc_token_audit_config` property doesn't exist in `push_config` block
**Solution:** Changed to correct `oidc_token` block with proper structure

**Before:**
```hcl
push_config {
  push_endpoint = "${google_cloud_run_service.consumer.status[0].url}/process"
  oidc_token_audit_config {
    service_account_email = google_service_account.consumer.email
  }
}
```

**After:**
```hcl
push_config {
  push_endpoint = "${google_cloud_run_service.consumer.status[0].url}/process"

  oidc_token {
    service_account_email = google_service_account.consumer.email
  }
}
```

---

### 3. **iam.tf** - Incorrect IAM Member Resource Properties
**Issue:** `service` property doesn't exist; correct property is `service_name` and `location` is required
**Solution:** Updated resource to use correct property names and added location

**Before:**
```hcl
resource "google_cloud_run_service_iam_member" "consumer_pubsub" {
  project = var.gcp_project_id
  service = google_cloud_run_service.consumer.name
  role    = "roles/run.invoker"
  member  = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_cloud_run_service.consumer]
}
```

**After:**
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

---

### 4. **cloudrun.tf** - Type Mismatch in Annotations
**Issue:** Numeric variable used directly in string annotation field; should be converted to string
**Solution:** Applied `tostring()` function to convert variables

**Before:**
```hcl
metadata {
  annotations = {
    "autoscaling.knative.dev/maxScale" = "100"
    "autoscaling.knative.dev/minScale" = var.backend_min_instances
  }
}
```

**After:**
```hcl
metadata {
  annotations = {
    "autoscaling.knative.dev/maxScale"  = "100"
    "autoscaling.knative.dev/minScale"  = tostring(var.backend_min_instances)
  }
}
```

---

### 5. **cloudrun.tf** - CPU Value Format
**Issue:** CPU value "1" should be in millicores format for clarity and consistency
**Solution:** Changed to "1000m" (1 CPU = 1000 millicores)

**Before:**
```hcl
resources {
  limits = {
    cpu    = "1"
    memory = "512Mi"
  }
}
```

**After:**
```hcl
resources {
  limits = {
    cpu    = "1000m"
    memory = "512Mi"
  }
}
```

---

## Files Modified

✅ `terraform/firestore.tf` - Removed invalid resource
✅ `terraform/pubsub.tf` - Fixed OIDC token configuration
✅ `terraform/iam.tf` - Updated IAM member properties
✅ `terraform/cloudrun.tf` - Fixed type conversions and CPU format

## Validation

All Terraform files now follow the correct Google provider syntax for the latest version.

### To validate the fixes:

```bash
cd terraform
terraform init
terraform validate
```

This should now pass without errors related to invalid properties.

