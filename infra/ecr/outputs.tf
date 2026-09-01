# What the other layers and CI consume.

output "repository_url" {
  description = "Registry URL for the repository, without a tag. compute's container_image is this joined to a tag, e.g. $${module.ecr.repository_url}:$${var.image_tag}."
  value       = module.ecr.repository_url
}

output "repository_arn" {
  description = "ARN of the repository, for IAM policies that grant pull or push access to it specifically rather than to every repository in the account."
  value       = module.ecr.repository_arn
}

output "repository_name" {
  description = "Name of the repository, for the CI job that tags and pushes images to it."
  value       = module.ecr.repository_name
}

output "registry_id" {
  description = "Account ID the registry belongs to, used by `aws ecr get-login-password` in CI."
  value       = module.ecr.repository_registry_id
}
