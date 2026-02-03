# Build backend image and push to Artifact Registry
resource "null_resource" "build_backend" {
  provisioner "local-exec" {
    command = "cd ${path.module}/../backend && gcloud builds submit --region=${var.gcp_region} --project=${var.gcp_project_id} ."
  }

  depends_on = [
    google_artifact_registry_repository.clicker,
    google_project_service.cloudbuild
  ]
}

# Build consumer image and push to Artifact Registry
resource "null_resource" "build_consumer" {
  provisioner "local-exec" {
    command = "cd ${path.module}/../consumer && gcloud builds submit --region=${var.gcp_region} --project=${var.gcp_project_id} ."
  }

  depends_on = [
    google_artifact_registry_repository.clicker,
    google_project_service.cloudbuild
  ]
}
