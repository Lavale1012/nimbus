# Version constraints for this module, matching networking/versions.tf and
# compute/versions.tf.
#
# Registry module versions are NOT captured by .terraform.lock.hcl — that file
# only locks providers. The `version` argument on the rds module block in main.tf
# is the only thing preventing a later `terraform init` from resolving a
# different major release, so it is pinned there.
terraform {
  # Same floor as the other modules: 1.9 is where a validation block may
  # reference another variable, which variables.tf here relies on.
  required_version = ">= 1.9"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
