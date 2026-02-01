resource "google_firestore_database" "clicker" {
  project     = var.gcp_project_id
  name        = var.firestore_database_id
  location_id = var.gcp_region
  type        = "FIRESTORE_NATIVE"

  # Allow deletion on terraform destroy
  delete_protection_state = "DELETE_PROTECTION_DISABLED"
  deletion_policy         = "DELETE"
}

# Note: Global counter document is created via init-firestore.sh script
# Terraform google provider doesn't have a native resource to manage documents
# Use the initialization script to set up the initial data
