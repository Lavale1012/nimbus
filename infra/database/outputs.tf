# What the other layers consume. Compute needs the endpoint, port and database
# name to build a DSN, and the master password secret ARN to resolve the
# credential at task launch without it ever appearing in the task definition.

################################################################################
# Connection
################################################################################

output "db_instance_endpoint" {
  description = "Connection endpoint in host:port form."
  value       = module.db.db_instance_endpoint
}

output "db_instance_address" {
  description = "Hostname of the instance, without the port."
  value       = module.db.db_instance_address
}

output "db_instance_port" {
  description = "Port the instance accepts connections on."
  value       = module.db.db_instance_port
}

output "db_instance_name" {
  description = "Name of the database inside the instance."
  value       = module.db.db_instance_name
}

output "db_instance_username" {
  description = "Master username."
  value       = module.db.db_instance_username
  sensitive   = true
}

################################################################################
# Credentials
#
# The password itself is deliberately not an output. RDS writes it to Secrets
# Manager and this is the ARN the ECS execution role reads it from, which is what
# keeps it out of the state file and out of the task definition.
################################################################################

output "db_instance_master_user_secret_arn" {
  description = "ARN of the Secrets Manager secret holding the generated master password. Pass this to compute's task_exec_secret_arns so the task can resolve the credential at launch."
  value       = module.db.db_instance_master_user_secret_arn
}

################################################################################
# Network
################################################################################

output "security_group_id" {
  description = "Security group attached to the database. Ingress is limited to the security groups passed in allowed_security_group_ids."
  value       = aws_security_group.db.id
}

output "db_subnet_group_id" {
  description = "DB subnet group the instance was placed in."
  value       = module.db.db_subnet_group_id
}

################################################################################
# Instance
################################################################################

output "db_instance_identifier" {
  description = "Identifier of the RDS instance."
  value       = module.db.db_instance_identifier
}

output "db_instance_availability_zone" {
  description = "Availability zone the single-AZ instance actually landed in."
  value       = module.db.db_instance_availability_zone
}

output "db_instance_engine_version_actual" {
  description = "Engine version AWS resolved, useful when engine_version is a major-only value such as \"17\"."
  value       = module.db.db_instance_engine_version_actual
}
