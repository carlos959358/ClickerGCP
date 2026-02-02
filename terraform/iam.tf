
resource "google_service_account" "backend" {
  project      = var.gcp_project_id
  account_id   = "clicker-backend"
  display_name = "Clicker Backend Service Account"
}

resource "google_service_account" "consumer" {
  project      = var.gcp_project_id
  account_id   = "clicker-consumer"
  display_name = "Clicker Consumer Service Account"
}

# Backend permissions
resource "google_project_iam_member" "backend_pubsub_publisher" {
  project = var.gcp_project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.backend.email}"

  depends_on = [google_service_account.backend]
}

resource "google_project_iam_member" "backend_firestore_editor" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.backend.email}"

  depends_on = [google_service_account.backend]
}

# Consumer permissions - Pub/Sub subscriber
resource "google_project_iam_member" "consumer_pubsub_subscriber" {
  project = var.gcp_project_id
  role    = "roles/pubsub.subscriber"
  member  = "serviceAccount:${google_service_account.consumer.email}"

  depends_on = [google_service_account.consumer]
}

# Consumer permissions - Pub/Sub viewer (needed to check subscription existence)
resource "google_project_iam_member" "consumer_pubsub_viewer" {
  project = var.gcp_project_id
  role    = "roles/pubsub.viewer"
  member  = "serviceAccount:${google_service_account.consumer.email}"

  depends_on = [google_service_account.consumer]
}

resource "google_project_iam_member" "consumer_firestore_editor" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.consumer.email}"

  depends_on = [google_service_account.consumer]
}

# Allow Pub/Sub to invoke consumer service
resource "google_cloud_run_service_iam_member" "consumer_pubsub" {
  project  = var.gcp_project_id
  service  = google_cloud_run_service.consumer.name
  location = var.gcp_region
  role     = "roles/run.invoker"
  member   = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_cloud_run_service.consumer]
}

# Get project number for Pub/Sub service account
data "google_project" "current" {
  project_id = var.gcp_project_id
}
