# What the other layers consume. Compute needs the first four to place tasks
# and register them behind the load balancer; Database needs the VPC and its
# private subnets to build a subnet group.

################################################################################
# Network
################################################################################

output "vpc_id" {
  description = "ID of the VPC, for security groups and the RDS subnet group."
  value       = module.vpc.vpc_id
}

output "vpc_cidr_block" {
  description = "CIDR block of the VPC."
  value       = module.vpc.vpc_cidr_block
}

output "private_subnets" {
  description = "Private subnet IDs. ECS tasks and RDS go here; no route to the internet gateway."
  value       = module.vpc.private_subnets
}

output "public_subnets" {
  description = "Public subnet IDs. Only the load balancer and NAT gateway belong here."
  value       = module.vpc.public_subnets
}

################################################################################
# Load balancer
################################################################################

output "alb_security_group_id" {
  description = "Security group attached to the load balancer. The ECS task security group should allow ingress from this ID rather than a CIDR, so it stays correct if subnet ranges change."
  value       = module.alb.security_group_id
}

output "target_group_arn" {
  description = "ARN of the ECS target group. The ECS service registers task IPs against this."
  value       = module.alb.target_groups["ecs_task"].arn
}

output "alb_dns_name" {
  description = "DNS name of the load balancer, for reaching it before DNS propagates."
  value       = module.alb.dns_name
}

output "alb_zone_id" {
  description = "Canonical hosted zone ID of the load balancer, for alias records in other zones."
  value       = module.alb.zone_id
}

################################################################################
# Application
################################################################################

output "app_url" {
  description = "Public URL the API is served from once DNS resolves."
  value       = "https://${var.domain_name}"
}

output "alb_logs_bucket" {
  description = "Bucket holding ALB access logs."
  value       = module.alb_logs.s3_bucket_id
}
