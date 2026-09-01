# Baseline tags every resource in this module carries, overridable per-caller
# via var.tags. Same shape as networking/ and compute/ so all three layers group
# identically in a cost report.
locals {
  tags = merge(
    {
      Terraform   = "true"
      Environment = var.environment
    },
    var.tags,
  )

  repository_name = coalesce(var.repository_name, var.app_name)
}

# The registry the ECS task pulls its image from.
#
# This is a separate module from compute/ on purpose. A repository holds build
# artifacts that outlive any particular deployment: `terraform destroy` on the
# service should not take the images with it, and the repository has to exist
# and already contain an image before an ECS service can start a task. Folding
# the two into one apply means the service's first create races an empty
# repository and fails the pull.
module "ecr" {
  source  = "terraform-aws-modules/ecr/aws"
  version = "~> 3.0"

  repository_name = local.repository_name

  # IMMUTABLE means a tag, once pushed, permanently refers to the same bytes.
  # It is what makes a rollback meaningful: redeploying a known-good tag gets
  # the image that tag was tested against, not whatever was pushed over it
  # since. The cost is that CI cannot re-push a moving `:latest` — images have
  # to be tagged by commit SHA or release version.
  repository_image_tag_mutability = var.image_tag_mutability

  # Scans on push for known CVEs in the image's OS packages, matching the
  # dependency scanning the CI pipeline already does for Go modules.
  repository_image_scan_on_push = var.scan_on_push

  # Same guard as networking's alb_logs_force_destroy: left false, a destroy
  # fails loudly on a repository that still holds images rather than silently
  # deleting every artifact in it.
  repository_force_delete = var.force_delete

  # Same-account pulls are authorized by the execution role's IAM policy, so
  # this stays empty until something outside the account needs to pull.
  repository_read_access_arns = var.read_access_arns

  # The module sets create_lifecycle_policy = true by default but leaves the
  # document empty, and AWS rejects an empty policy at apply time — so this is
  # required rather than optional.
  #
  # Rules are evaluated in rulePriority order and an image matched by one rule
  # is not considered by any later rule, which is why the catch-all sits last.
  repository_lifecycle_policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images after ${var.untagged_expire_days} days"
        # Untagged images are the layers orphaned when a tag is re-pushed. They
        # are referenced by nothing and billed per GB-month like everything else.
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = var.untagged_expire_days
        }
        action = { type = "expire" }
      },
      {
        rulePriority = 2
        description  = "Keep only the ${var.max_image_count} most recent images"
        # tagStatus "any" is the catch-all, so it has to be the last rule.
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = var.max_image_count
        }
        action = { type = "expire" }
      },
    ]
  })

  tags = local.tags
}
