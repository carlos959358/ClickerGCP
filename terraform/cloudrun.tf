resource "google_cloud_run_service" "backend" {
  project = var.gcp_project_id
  name    = var.backend_service_name
  location = var.gcp_region

  template {
    spec {
      service_account_name = google_service_account.backend.email

      containers {
        image = var.backend_docker_image

        env {
          name  = "GCP_PROJECT_ID"
          value = var.gcp_project_id
        }

        env {
          name  = "PUBSUB_TOPIC"
          value = google_pubsub_topic.click_events.name
        }

        env {
          name  = "FIRESTORE_DATABASE"
          value = google_firestore_database.clicker.name
        }

        resources {
          limits = {
            cpu    = "1000m"
            memory = var.backend_memory
          }
        }
      }

      timeout_seconds = var.request_timeout
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale"      = tostring(var.backend_max_instances)
        "autoscaling.knative.dev/minScale"      = tostring(var.backend_min_instances)
        "run.googleapis.com/cpu-throttling"     = "true"
        "run.googleapis.com/startup-cpu-boost"  = "false"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  depends_on = [
    google_project_service.run,
    google_service_account.backend,
    google_project_iam_member.backend_pubsub_publisher,
    google_project_iam_member.backend_firestore_editor,
    null_resource.build_backend,
  ]
}

resource "google_cloud_run_service" "consumer" {
  project = var.gcp_project_id
  name    = var.consumer_service_name
  location = var.gcp_region

  template {
    spec {
      service_account_name = google_service_account.consumer.email

      containers {
        image = var.consumer_docker_image

        env {
          name  = "GCP_PROJECT_ID"
          value = var.gcp_project_id
        }

        env {
          name  = "PUBSUB_SUBSCRIPTION"
          value = var.pubsub_subscription_name
        }

        env {
          name  = "FIRESTORE_DATABASE"
          value = google_firestore_database.clicker.name
        }

        env {
          name  = "BACKEND_URL"
          value = google_cloud_run_service.backend.status[0].url
        }

        resources {
          limits = {
            cpu    = "1000m"
            memory = var.consumer_memory
          }
        }
      }

      timeout_seconds = var.request_timeout
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale"      = tostring(var.consumer_max_instances)
        "autoscaling.knative.dev/minScale"      = tostring(var.consumer_min_instances)
        "run.googleapis.com/cpu-throttling"     = "true"
        "run.googleapis.com/startup-cpu-boost"  = "false"
      }
    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }

  depends_on = [
    google_project_service.run,
    google_service_account.consumer,
    google_project_iam_member.consumer_pubsub_subscriber,
    google_project_iam_member.consumer_firestore_editor,
    null_resource.build_consumer,
  ]
}

resource "google_cloud_run_service_iam_member" "backend_public" {
  project = var.gcp_project_id
  service = google_cloud_run_service.backend.name
  role    = "roles/run.invoker"
  member  = "allUsers"
}
