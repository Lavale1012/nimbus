# Baseline tags every resource in this module carries, overridable per-caller
# via var.tags. Same shape as networking/, compute/ and database/ so a cost
# report can group every layer by the same keys.
locals {
  tags = merge(
    {
      Terraform   = "true"
      Environment = var.environment
    },
    var.tags,
  )

  bucket_name = coalesce(var.bucket_name, "${var.app_name}-user-files")
}

################################################################################
# The bucket
#
# Holds user-uploaded file bytes — the most sensitive store in the project, more
# so than RDS, which only holds metadata about them. The application never
# proxies these bytes: it signs a 15-minute URL and the CLI transfers directly.
################################################################################

module "user_files" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 5.15.0"

  bucket = local.bucket_name

  # ACLs disabled entirely — the bucket owner owns every object regardless of
  # who wrote it. This is why there is no `acl` argument: setting one alongside
  # BucketOwnerEnforced is rejected, not ignored.
  control_object_ownership = true
  object_ownership         = "BucketOwnerEnforced"

  # The module already defaults all four to true. Set explicitly because this
  # bucket holds user data, and these are the four lines a reviewer looks for
  # rather than something to be inferred from an upstream default.
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true

  server_side_encryption_configuration = {
    rule = {
      apply_server_side_encryption_by_default = {
        sse_algorithm = "AES256"
      }
    }
  }

  # Denies any request arriving over plain HTTP. A presigned URL is just a
  # signed URL — nothing about it forces TLS — so without this policy a client
  # can transfer a user's files in the clear. Same reasoning as rds.force_ssl
  # on the database.
  attach_deny_insecure_transport_policy = true
  attach_require_latest_tls_policy      = true

  versioning = {
    enabled = var.versioning_enabled
  }

  # All three rules are declared unconditionally rather than switched on
  # var.versioning_enabled. The two version-related rules simply never match on
  # an unversioned bucket — there are no noncurrent versions or delete markers
  # for them to act on — so gating them buys nothing, and leaving them in place
  # means enabling versioning later cannot produce an unbounded bucket by
  # someone forgetting the expiry half of the change.
  lifecycle_rule = [
    # Bounds what versioning costs. Without this every overwritten and deleted
    # byte is billed indefinitely, on a bucket whose entire purpose is user
    # files that grow without bound.
    {
      id      = "expire-noncurrent-versions"
      enabled = true
      noncurrent_version_expiration = {
        noncurrent_days = var.noncurrent_version_retention_days
      }
    },
    # Once the last noncurrent version expires, the delete marker is left behind
    # with nothing under it. Needs its own rule: expired_object_delete_marker
    # cannot share an expiration block with `days`.
    {
      id      = "expire-orphaned-delete-markers"
      enabled = true
      expiration = {
        expired_object_delete_marker = true
      }
    },
    # A failed multipart upload leaves parts that are billed indefinitely and
    # never appear in the console object listing. Unrelated to versioning —
    # this is a pure leak either way.
    {
      id                                     = "abort-incomplete-multipart-uploads"
      enabled                                = true
      abort_incomplete_multipart_upload_days = var.abort_incomplete_upload_days
    },
  ]

  force_destroy = var.force_destroy

  tags = local.tags
}

################################################################################
# Task access
#
# Lives here rather than in iam/ so the grant sits next to the resource it
# grants on and the two cannot drift.
#
# This attaches to the ECS *task* role, not the execution role. Presigned URLs
# carry the signer's authority, so every permission the CLI exercises against
# an upload or download URL is a permission the task role must already hold.
################################################################################

data "aws_iam_policy_document" "task_access" {
  statement {
    sid     = "ListBucketForHealthCheck"
    actions = ["s3:ListBucket"]
    # The bucket ARN, with no /* — ListBucket is a bucket-level action. Pointed
    # at the object ARN instead it fails, and it surfaces as /health returning
    # 503 rather than as an obvious permissions error.
    resources = [module.user_files.s3_bucket_arn]
  }

  statement {
    sid = "ReadWriteUserObjects"
    # CopyObject, used by the rename handler, needs no action of its own: it is
    # GetObject on the source plus PutObject on the destination.
    #
    # s3:DeleteObjectVersion is deliberately absent. On a versioned bucket a
    # plain DeleteObject writes a delete marker rather than destroying bytes,
    # which is all the handler needs. Version-level delete would let a
    # compromised task erase the history versioning exists to preserve;
    # reclaiming that storage is the lifecycle rule's job, not the app's.
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
    ]
    resources = ["${module.user_files.s3_bucket_arn}/*"]
  }
}

resource "aws_iam_policy" "task_access" {
  name_prefix = "${var.app_name}-s3-task-"
  description = "Read and write access to ${local.bucket_name} for the API task"
  policy      = data.aws_iam_policy_document.task_access.json

  tags = local.tags
}
