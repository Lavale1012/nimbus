# .terraform.lock.hcl locks providers only, NOT registry module versions. The
# `version` arguments on each module block in main.tf are the only thing
# stopping a later `terraform init` from resolving a different major release.
terraform {
  # 1.9 is the floor for referencing another variable inside a validation block,
  # which variables.tf does to reject conflicting NAT settings.
  required_version = ">= 1.9"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
