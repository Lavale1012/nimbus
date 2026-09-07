# What the other layers consume. Compute needs the bucket name for the S3_BUCKET
# environment variable and the policy ARN to attach to the ECS task role; without
# that attachment every presigned upload and download returns 403, because a
# presigned URL carries only the authority the signer already had.

################################################################################
# Bucket
################################################################################

output "s3_bucket_id" {
  description = "Name of the bucket. This is the value the application reads as S3_BUCKET."
  value       = module.user_files.s3_bucket_id
}

output "s3_bucket_arn" {
  description = "ARN of the bucket."
  value       = module.user_files.s3_bucket_arn
}

output "s3_bucket_regional_domain_name" {
  description = "Regional domain name of the bucket, the host presigned URLs are issued against."
  value       = module.user_files.s3_bucket_bucket_regional_domain_name
}

output "s3_bucket_region" {
  description = "Region the bucket lives in. Presigned URLs are region-specific, so the application must sign against this one."
  value       = module.user_files.s3_bucket_region
}

################################################################################
# Access
################################################################################

output "task_access_policy_arn" {
  description = "IAM policy granting read/write on this bucket plus the ListBucket the /health check needs. Attach to the ECS task role — the task role, not the execution role, is what signs presigned URLs."
  value       = aws_iam_policy.task_access.arn
}
