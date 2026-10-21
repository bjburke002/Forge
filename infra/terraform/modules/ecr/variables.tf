variable "prefix" {
  type        = string
  default     = "forge"
  description = "Repository namespace. Pushes to <prefix>/<name> create repos from the template defined in this module."
}

variable "untagged_expiry_days" {
  type    = number
  default = 7
}

variable "max_tagged_images" {
  type        = number
  default     = 20
  description = "Build images (sha- tags) to keep per repository."
}