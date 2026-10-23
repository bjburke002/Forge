variable "domain_name" {
  type        = string
  description = "Root domain name for which you are creating the certificate. Module creates the wildcard with this."

  validation {
    condition     = !startswith(var.domain_name, "*")
    error_message = "domain_name should be the root domain name."
  }
}

variable "subject_alternative_names" {
  type        = list(string)
  default     = []
  description = "List of alternative names for the cert outside of root domain and wildcard."
}

variable "route53_zone_id" {
  type        = string
  description = "The DNS zone ID."
}