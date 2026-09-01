# Same principle as the other modules: environment-shaping choices carry no
# default, so the decision lives in the caller's config where a human reviews it.

variable "app_name" {
  type        = string
  description = "Name of the application. The repository name derives from it unless overridden below."
}

variable "environment" {
  type        = string
  description = "Deployment environment the repository belongs to (e.g. dev, staging, prod). Becomes the Environment tag."
}

variable "tags" {
  type        = map(string)
  description = "Additional tags applied to all resources created by the module, merged over the Terraform/Environment baseline."
}

variable "repository_name" {
  type        = string
  default     = null
  description = "Explicit repository name. Leave null to use app_name. Renaming a repository creates a new one and orphans every image in the old, so this exists to match a registry that already holds images."

  validation {
    condition     = var.repository_name == null || can(regex("^[a-z0-9][a-z0-9._/-]*$", coalesce(var.repository_name, "x")))
    error_message = "ECR repository names must be lowercase and may contain only letters, numbers, and the characters . _ - / — an uppercase name is rejected by AWS at apply time."
  }
}

################################################################################
# Image handling
################################################################################

variable "image_tag_mutability" {
  type        = string
  default     = "IMMUTABLE"
  description = "Whether a pushed tag can be overwritten. IMMUTABLE is what makes a rollback trustworthy: redeploying a tag gets the bytes it was tested against. Requires CI to tag by commit SHA or version rather than re-pushing a moving :latest."

  validation {
    condition     = contains(["MUTABLE", "MUTABLE_WITH_EXCLUSION", "IMMUTABLE", "IMMUTABLE_WITH_EXCLUSION"], var.image_tag_mutability)
    error_message = "image_tag_mutability must be one of: MUTABLE, MUTABLE_WITH_EXCLUSION, IMMUTABLE, IMMUTABLE_WITH_EXCLUSION."
  }
}

variable "scan_on_push" {
  type        = bool
  default     = true
  description = "Scan images for known CVEs on push. Cheap, and it covers the OS packages in the image that Go module scanning in CI cannot see."
}

variable "force_delete" {
  type        = bool
  default     = false
  description = "Allow the repository to be destroyed while it still holds images. Leave false outside throwaway environments; a destroy will otherwise fail rather than silently deleting every build artifact."
}

variable "read_access_arns" {
  type        = list(string)
  default     = []
  description = "Principal ARNs granted pull access via a repository policy. Same-account pulls are already authorized by the ECS execution role's IAM policy, so this stays empty until something outside the account needs to pull."
}

################################################################################
# Lifecycle policy
#
# Without these rules a repository grows forever at per-GB-month pricing — the
# same failure mode as log retention in networking/ and compute/.
################################################################################

variable "untagged_expire_days" {
  type        = number
  default     = 7
  description = "Days before an untagged image is expired. Untagged images are layers orphaned by a re-push; nothing references them."

  validation {
    condition     = var.untagged_expire_days > 0
    error_message = "untagged_expire_days must be greater than zero."
  }
}

variable "max_image_count" {
  type        = number
  default     = 30
  description = "Number of images to retain before the oldest are expired. Keep this comfortably above the number of deploys in your rollback window: the rule deletes by age, and a Fargate task re-pulls its image on restart, so expiring the image a running service uses breaks that task's next start as well as the rollback."

  validation {
    condition     = var.max_image_count >= 5
    error_message = "max_image_count below 5 leaves almost no rollback history and risks expiring an image a running task still needs to re-pull."
  }
}
