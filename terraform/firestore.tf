resource "google_firestore_database" "clicker" {
  project     = var.gcp_project_id
  name        = var.firestore_database_id
  location_id = var.gcp_region
  type        = "FIRESTORE_NATIVE"

  # Allow deletion on terraform destroy
  delete_protection_state = "DELETE_PROTECTION_DISABLED"
  deletion_policy         = "DELETE"

  # Wait for builds to complete before creating database
  # This gives Firestore time to clean up recently deleted database IDs
  depends_on = [
    null_resource.build_backend,
    null_resource.build_consumer,
    google_project_service.firestore
  ]
}

# Note: Global counter document is created via init-firestore.sh script
# Terraform google provider doesn't have a native resource to manage documents
# Use the initialization script to set up the initial data
