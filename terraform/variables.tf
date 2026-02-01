variable "gcp_project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "gcp_region" {
  description = "GCP region"
  type        = string
  default     = "europe-southwest1"
}

variable "backend_service_name" {
  description = "Backend Cloud Run service name"
  type        = string
  default     = "clicker-backend"
}

variable "consumer_service_name" {
  description = "Consumer Cloud Run service name"
  type        = string
  default     = "clicker-consumer"
}

variable "pubsub_topic_name" {
  description = "Pub/Sub topic name for click events"
  type        = string
  default     = "click-events"
}

variable "pubsub_subscription_name" {
  description = "Pub/Sub subscription name for consumer"
  type        = string
  default     = "click-consumer-sub"
}

variable "firestore_database_id" {
  description = "Firestore database ID"
  type        = string
  default     = "clicker-db"
}

variable "backend_docker_image" {
  description = "Docker image URL for backend service"
  type        = string
}

variable "consumer_docker_image" {
  description = "Docker image URL for consumer service"
  type        = string
}

variable "backend_min_instances" {
  description = "Minimum instances for backend (0 = no persistent instance)"
  type        = number
  default     = 1
}

variable "consumer_min_instances" {
  description = "Minimum instances for consumer"
  type        = number
  default     = 1
}

variable "backend_max_instances" {
  description = "Maximum instances for backend (free tier: 10)"
  type        = number
  default     = 10
}

variable "consumer_max_instances" {
  description = "Maximum instances for consumer (free tier: 5)"
  type        = number
  default     = 5
}

variable "backend_memory" {
  description = "Memory allocation for backend"
  type        = string
  default     = "256Mi"
}

variable "consumer_memory" {
  description = "Memory allocation for consumer"
  type        = string
  default     = "256Mi"
}

variable "request_timeout" {
  description = "Request timeout in seconds"
  type        = number
  default     = 60
}

# GitHub Configuration for Cloud Build
variable "github_owner" {
  description = "GitHub repository owner (username)"
  type        = string
}

variable "github_repo" {
  description = "GitHub repository name"
  type        = string
}
