# Reference existing Artifact Registry Repository
data "google_artifact_registry_repository" "clicker" {
  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "clicker-repo"

  depends_on = [google_project_service.artifactregistry]
}

# Build backend image and push to Artifact Registry
resource "null_resource" "build_backend" {
  provisioner "local-exec" {
    command = "cd ${path.module}/../backend && gcloud builds submit --region=${var.gcp_region} --project=${var.gcp_project_id} ."
  }

  depends_on = [
    data.google_artifact_registry_repository.clicker,
    google_project_service.cloudbuild
  ]
}

# Build consumer image and push to Artifact Registry
resource "null_resource" "build_consumer" {
  provisioner "local-exec" {
    command = "cd ${path.module}/../consumer && gcloud builds submit --region=${var.gcp_region} --project=${var.gcp_project_id} ."
  }

  depends_on = [
    data.google_artifact_registry_repository.clicker,
    google_project_service.cloudbuild
  ]
}
