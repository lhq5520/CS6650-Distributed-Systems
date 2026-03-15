variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "project_name" {
  description = "Project name used as prefix for all resources"
  type        = string
  default     = "hw7"
}

variable "ecr_image_uri" {
  description = "ECR image URI for the order service"
  type        = string
}

variable "num_workers" {
  description = "Number of SQS worker goroutines (Phase 5 tuning)"
  type        = number
  default     = 1
}
