variable "region" {
  type    = string
  default = "us-west-2"
}

variable "project_name" {
  type    = string
  default = "crash-recovery"
}

variable "search_image_uri" {
  type        = string
  description = "ECR image URI for search service (e.g. 123456.dkr.ecr.us-west-2.amazonaws.com/search-service:latest)"
}

variable "rec_image_uri" {
  type        = string
  description = "ECR image URI for recommendation service (e.g. 123456.dkr.ecr.us-west-2.amazonaws.com/rec-service:latest)"
}

variable "resilience_enabled" {
  type    = bool
  default = false
}
