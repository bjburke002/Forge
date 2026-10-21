variable "repo_url" {
  type        = string
  description = "URL of the repo that where the application lives."
}

variable "path" {
  type        = string
  description = "Path to the manifests"
}

variable "revision" {
  type        = string
  description = "Branch, e.g. main"
}