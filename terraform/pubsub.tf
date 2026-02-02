resource "google_pubsub_topic" "click_events" {
  project = var.gcp_project_id
  name    = var.pubsub_topic_name

  message_retention_duration = "600s"
}

resource "google_pubsub_subscription" "click_consumer" {
  project = var.gcp_project_id
  name    = var.pubsub_subscription_name
  topic   = google_pubsub_topic.click_events.name

  ack_deadline_seconds = 60

  push_config {
    push_endpoint = "${google_cloud_run_service.consumer.status[0].url}/process"

    oidc_token {
      service_account_email = google_service_account.consumer.email
      audience              = google_cloud_run_service.consumer.status[0].url
    }
  }

  depends_on = [
    google_pubsub_topic.click_events,
    google_cloud_run_service.consumer
  ]
}
