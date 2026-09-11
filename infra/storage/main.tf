# Same tag shape as networking/, compute/ and database/ so cost reports can
# group every layer by the same keys.
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
# Holds user-uploaded bytes — more sensitive than RDS, which only holds
# metadata about them. The app never proxies the bytes: it signs a 15-minute
# URL and the CLI transfers directly.
################################################################################

module "user_files" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 5.15.0"

  bucket = local.bucket_name

  # ACLs disabled — the bucket owner owns every object. Hence no `acl`
  # argument: setting one alongside BucketOwnerEnforced is rejected, not
  # ignored.
  control_object_ownership = true
  object_ownership         = "BucketOwnerEnforced"

  # Already the module defaults, but stated explicitly: a reviewer should not
  # have to infer them from an upstream default on a bucket of user data.
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

  # Nothing about a presigned URL forces TLS, so without this a client can
  # transfer a user's files in the clear. Same reasoning as rds.force_ssl.
  attach_deny_insecure_transport_policy = true
  attach_require_latest_tls_policy      = true

  versioning = {
    enabled = var.versioning_enabled
  }

  # Unconditional rather than gated on var.versioning_enabled: the version rules
  # never match on an unversioned bucket, so gating buys nothing, and leaving
  # them in place means enabling versioning later cannot silently go unbounded.
  lifecycle_rule = [
    # Bounds what versioning costs — otherwise every overwritten and deleted
    # byte is billed indefinitely.
    {
      id      = "expire-noncurrent-versions"
      enabled = true
      noncurrent_version_expiration = {
        noncurrent_days = var.noncurrent_version_retention_days
      }
    },
    # The last expiring version leaves a delete marker with nothing under it.
    # Needs its own rule: expired_object_delete_marker cannot share an
    # expiration block with `days`.
    {
      id      = "expire-orphaned-delete-markers"
      enabled = true
      expiration = {
        expired_object_delete_marker = true
      }
    },
    # Failed multipart uploads leave parts that are billed indefinitely and
    # never show in the object listing. A leak with or without versioning.
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
# Lives here, not in iam/, so the grant cannot drift from the resource it grants
# on. Attaches to the ECS *task* role, not the execution role: presigned URLs
# carry the signer's authority, so anything the CLI does against a URL is a
# permission the task role must already hold.
################################################################################

data "aws_iam_policy_document" "task_access" {
  statement {
    sid     = "ListBucketForHealthCheck"
    actions = ["s3:ListBucket"]
    # Bucket ARN with no /* — ListBucket is bucket-level. The object ARN fails
    # here, and surfaces as /health returning 503, not as a permissions error.
    resources = [module.user_files.s3_bucket_arn]
  }

  statement {
    sid = "ReadWriteUserObjects"
    # CopyObject (the rename handler) needs no action of its own — it is
    # GetObject on the source plus PutObject on the destination.
    #
    # s3:DeleteObjectVersion is deliberately absent: plain DeleteObject writes a
    # delete marker, which is all the handler needs, and version-level delete
    # would let a compromised task erase the history versioning exists to keep.
    # Reclaiming that storage is the lifecycle rule's job.
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
