# As in the other modules: environment-shaping choices carry no default, so the
# decision stays in the caller's config. Defaults appear only where the value is
# a safe floor or the protective option.

variable "app_name" {
  type        = string
  description = "Name of the application. The bucket name and IAM policy name derive from it unless overridden below."
}

variable "environment" {
  type        = string
  description = "Deployment environment the bucket belongs to (e.g. dev, staging, prod). Becomes the Environment tag."
}

variable "tags" {
  type        = map(string)
  description = "Additional tags applied to all resources created by the module, merged over the Terraform/Environment baseline."
}

################################################################################
# Naming
################################################################################

variable "bucket_name" {
  type        = string
  default     = null
  description = "Explicit bucket name. Leave null to derive it as <app_name>-user-files. S3 bucket names are globally unique across all AWS accounts, so a derived name can still collide with a stranger's."
}

################################################################################
# Versioning and lifecycle
################################################################################

variable "versioning_enabled" {
  type        = bool
  default     = true
  description = "Keep previous versions of an object when it is overwritten or deleted. Without it, a presigned PUT reusing an existing key destroys the previous file permanently. Note S3 only allows suspending versioning once enabled, never disabling it, and suspension leaves existing versions in place and billed — this is effectively a one-way switch."
}

variable "noncurrent_version_retention_days" {
  type        = number
  default     = 30
  description = "Days to keep a noncurrent version before expiring it. This is the only thing bounding what versioning costs; every overwritten and deleted byte is stored for this long."

  validation {
    condition     = !var.versioning_enabled || var.noncurrent_version_retention_days > 0
    error_message = "Versioning is enabled, so noncurrent_version_retention_days must be greater than zero. A versioned bucket with no expiry keeps every overwritten and deleted object forever, which on user file storage is an unbounded bill that surfaces months later."
  }
}

variable "abort_incomplete_upload_days" {
  type        = number
  default     = 7
  description = "Days before an incomplete multipart upload is aborted and its parts deleted. Orphaned parts are billed indefinitely and never appear in an object listing, so this applies whether or not versioning is on."

  validation {
    condition     = var.abort_incomplete_upload_days > 0
    error_message = "abort_incomplete_upload_days must be greater than zero, otherwise failed uploads leak storage that is invisible in the console."
  }
}

################################################################################
# Protection
################################################################################

variable "force_destroy" {
  type        = bool
  default     = false
  description = "Allow the bucket to be destroyed while it still holds objects. Leave false outside throwaway environments: this bucket holds user files, and with versioning on a force destroy purges every version, not just current objects."
}
