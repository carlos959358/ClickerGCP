
resource "google_service_account" "backend" {
  project      = var.gcp_project_id
  account_id   = "clicker-backend"
  display_name = "Clicker Backend Service Account"
  description  = "Publishes click events to Pub/Sub, reads counters from Firestore"
}

resource "google_service_account" "consumer" {
  project      = var.gcp_project_id
  account_id   = "clicker-consumer"
  display_name = "Clicker Consumer Service Account"
  description  = "Consumes messages from Pub/Sub, updates counters in Firestore"
}

# ═══════════════════════════════════════════════════════════════════════════
# BACKEND SERVICE PERMISSIONS
# ═══════════════════════════════════════════════════════════════════════════
# Purpose: Publish click events to Pub/Sub topic
resource "google_project_iam_member" "backend_pubsub_publisher" {
  project = var.gcp_project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.backend.email}"

  depends_on = [google_service_account.backend]
}

# Purpose: Read counter data from Firestore
resource "google_project_iam_member" "backend_firestore_reader" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.backend.email}"

  depends_on = [google_service_account.backend]
}

# ═══════════════════════════════════════════════════════════════════════════
# CONSUMER SERVICE PERMISSIONS
# ═══════════════════════════════════════════════════════════════════════════
# Purpose: Subscribe and receive messages from Pub/Sub
resource "google_project_iam_member" "consumer_pubsub_subscriber" {
  project = var.gcp_project_id
  role    = "roles/pubsub.subscriber"
  member  = "serviceAccount:${google_service_account.consumer.email}"

  depends_on = [google_service_account.consumer]
}

# Purpose: View subscription details
resource "google_project_iam_member" "consumer_pubsub_viewer" {
  project = var.gcp_project_id
  role    = "roles/pubsub.viewer"
  member  = "serviceAccount:${google_service_account.consumer.email}"

  depends_on = [google_service_account.consumer]
}

# Purpose: Read and write counter data to Firestore
resource "google_project_iam_member" "consumer_firestore_editor" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.consumer.email}"

  depends_on = [google_service_account.consumer]
}

# ═══════════════════════════════════════════════════════════════════════════
# CLOUD RUN SERVICE IAM
# ═══════════════════════════════════════════════════════════════════════════
# Backend: INTERNAL ONLY - No public access
# (Backend is called only by frontend, not publicly exposed)
# Note: allUsers invoker role is NOT added to backend

# Consumer: Allow Pub/Sub to invoke via push delivery
# Security: The /process endpoint validates Pub/Sub message format (HMAC/JWT)
resource "google_cloud_run_service_iam_member" "consumer_pubsub_invoker" {
  project  = var.gcp_project_id
  service  = google_cloud_run_service.consumer.name
  location = var.gcp_region
  role     = "roles/run.invoker"
  member   = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_cloud_run_service.consumer]
}

# Also allow allUsers for Pub/Sub push (Pub/Sub may not always send OIDC token)
resource "google_cloud_run_service_iam_member" "consumer_public" {
  project  = var.gcp_project_id
  service  = google_cloud_run_service.consumer.name
  location = var.gcp_region
  role     = "roles/run.invoker"
  member   = "allUsers"

  depends_on = [google_cloud_run_service.consumer]
}

# Get project number for Pub/Sub service account
data "google_project" "current" {
  project_id = var.gcp_project_id
}
