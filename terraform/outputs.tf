output "backend_url" {
  description = "URL of the backend Cloud Run service"
  value       = google_cloud_run_service.backend.status[0].url
}

output "consumer_url" {
  description = "URL of the consumer Cloud Run service"
  value       = google_cloud_run_service.consumer.status[0].url
}

output "pubsub_topic_name" {
  description = "Pub/Sub topic name"
  value       = google_pubsub_topic.click_events.name
}

output "firestore_database_name" {
  description = "Firestore database name"
  value       = google_firestore_database.clicker.name
}

output "backend_service_account" {
  description = "Backend service account email"
  value       = google_service_account.backend.email
}

output "consumer_service_account" {
  description = "Consumer service account email"
  value       = google_service_account.consumer.email
}
