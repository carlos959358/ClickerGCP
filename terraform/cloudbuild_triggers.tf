# Cloud Build triggers for continuous deployment from GitHub

# Backend trigger - builds and pushes image on main branch push
resource "google_cloudbuild_trigger" "backend" {
  project     = var.gcp_project_id
  name        = "build-backend"
  description = "Build and push backend Docker image on GitHub push to main"

  github {
    owner = var.github_owner
    name  = var.github_repo
    push {
      branch = "^main$"
    }
  }

  included_files = ["backend/**"]

  build {
    step {
      name = "gcr.io/cloud-builders/docker"
      args = [
        "build",
        "-t", "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/backend:latest",
        "-t", "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/backend:$SHORT_SHA",
        "-f", "backend/Dockerfile",
        "backend/"
      ]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      args = [
        "push",
        "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/backend:latest"
      ]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      args = [
        "push",
        "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/backend:$SHORT_SHA"
      ]
    }
    images = ["${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/backend:latest"]
  }

  depends_on = [
    google_artifact_registry_repository.clicker,
    google_project_service.cloudbuild
  ]
}

# Consumer trigger - builds and pushes image on main branch push
resource "google_cloudbuild_trigger" "consumer" {
  project     = var.gcp_project_id
  name        = "build-consumer"
  description = "Build and push consumer Docker image on GitHub push to main"

  github {
    owner = var.github_owner
    name  = var.github_repo
    push {
      branch = "^main$"
    }
  }

  included_files = ["consumer/**"]

  build {
    step {
      name = "gcr.io/cloud-builders/docker"
      args = [
        "build",
        "-t", "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/consumer:latest",
        "-t", "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/consumer:$SHORT_SHA",
        "-f", "consumer/Dockerfile",
        "consumer/"
      ]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      args = [
        "push",
        "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/consumer:latest"
      ]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      args = [
        "push",
        "${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/consumer:$SHORT_SHA"
      ]
    }
    images = ["${var.gcp_region}-docker.pkg.dev/${var.gcp_project_id}/clicker-repo/consumer:latest"]
  }

  depends_on = [
    google_artifact_registry_repository.clicker,
    google_project_service.cloudbuild
  ]
}
