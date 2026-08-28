# Version constraints for this module.
#
# Registry module versions are NOT captured by .terraform.lock.hcl — that file
# only locks providers. The `version` arguments on each module block in main.tf
# are the only thing preventing a later `terraform init` from resolving a
# different major release, so they are pinned to the patch level.
terraform {
  # 1.9 is the floor for referencing another variable inside a validation
  # block, which variables.tf does to reject conflicting NAT settings.
  required_version = ">= 1.9"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
