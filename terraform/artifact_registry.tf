# Create Artifact Registry repository for Docker images
resource "google_artifact_registry_repository" "clicker" {
  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "clicker-repo"
  description   = "Docker images for ClickerGCP services"
  format        = "DOCKER"

  depends_on = [google_project_service.artifactregistry]
}
