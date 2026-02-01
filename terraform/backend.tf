terraform {
  backend "gcs" {
    bucket = "dev-trail-475809-v2-terraform-state"
    prefix = "clicker"
  }
}
