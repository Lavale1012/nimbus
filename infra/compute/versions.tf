# Version constraints for this module, matching networking/versions.tf.
#
# Registry module versions are NOT captured by .terraform.lock.hcl — that file
# only locks providers. The `version` argument on the ecs module block in main.tf
# is the only thing preventing a later `terraform init` from resolving a
# different major release, so it is pinned there.
terraform {
  # Same floor as networking/: 1.9 is where a validation block may reference
  # another variable, and it is the version this repo's modules are written for.
  required_version = ">= 1.9"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
