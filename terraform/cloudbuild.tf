# Google Cloud Build Configuration
# Automatically triggers build and deployment on push to main branch

# Enable Cloud Build API
resource "google_project_service" "cloudbuild" {
  project = var.gcp_project_id
  service = "cloudbuild.googleapis.com"
}

# Cloud Build Trigger for GitHub
resource "google_cloudbuild_trigger" "github_main" {
  project = var.gcp_project_id
  name    = "clicker-gcp-github-main"

  description = "Automatic CI/CD: Build and deploy on push to main branch"

  # GitHub connection
  github {
    owner = var.github_owner
    name  = var.github_repo
    push {
      branch = "^main$"
    }
  }

  # Use cloudbuild.yaml from repository root
  filename = "cloudbuild.yaml"

  # Run the trigger when PR is merged or push to main
  included_files = []
  ignored_files  = []

  depends_on = [
    google_project_service.cloudbuild,
  ]
}

# Cloud Build Service Account (already exists, but we'll ensure it has permissions)
locals {
  cloud_build_sa_email = "${data.google_project.current.number}@cloudbuild.gserviceaccount.com"
}

# Get current project details
data "google_project" "current" {
  project_id = var.gcp_project_id
}

# Grant Cloud Build service account necessary permissions

# Editor role (broad permissions for building and deploying)
resource "google_project_iam_member" "cloud_build_editor" {
  project = var.gcp_project_id
  role    = "roles/editor"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}

# Cloud Run Admin - deploy services
resource "google_project_iam_member" "cloud_build_run_admin" {
  project = var.gcp_project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}

# Service Account Admin - create/manage service accounts
resource "google_project_iam_member" "cloud_build_sa_admin" {
  project = var.gcp_project_id
  role    = "roles/iam.serviceAccountAdmin"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}

# Artifact Registry - push/pull images
resource "google_project_iam_member" "cloud_build_artifact_registry" {
  project = var.gcp_project_id
  role    = "roles/artifactregistry.admin"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}

# Firestore Admin
resource "google_project_iam_member" "cloud_build_firestore" {
  project = var.gcp_project_id
  role    = "roles/firestore.admin"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}

# Pub/Sub Admin
resource "google_project_iam_member" "cloud_build_pubsub" {
  project = var.gcp_project_id
  role    = "roles/pubsub.admin"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}

# Compute Admin (for infrastructure)
resource "google_project_iam_member" "cloud_build_compute" {
  project = var.gcp_project_id
  role    = "roles/compute.admin"
  member  = "serviceAccount:${local.cloud_build_sa_email}"
}
