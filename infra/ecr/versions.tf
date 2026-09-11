# .terraform.lock.hcl locks providers only, NOT registry module versions. The
# `version` argument on the ecr module block in main.tf is the only thing
# stopping a later `terraform init` from resolving a different major release.
terraform {
  # 1.9 is where a validation block may reference another variable, and the
  # version this repo is written for.
  required_version = ">= 1.9"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}
