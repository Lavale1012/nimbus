# Same tag shape as networking/local.tags and compute/local.tags so cost reports
# can group all three layers by the same keys.
locals {
  tags = merge(
    {
      Terraform   = "true"
      Environment = var.environment
    },
    var.tags,
  )

  identifier = coalesce(var.identifier, "${var.app_name}-db")

  # Derived from the engine version rather than passed separately: they are one
  # decision written twice, and when they disagree — family "postgres16"
  # against engine_version "17" — AWS rejects it mid-apply, after other
  # resources already exist.
  family = coalesce(var.family, "postgres${split(".", var.engine_version)[0]}")

  # Parameters this module guarantees, ahead of anything the caller adds.
  #
  # Concatenated rather than defaulted into var.parameters because a Terraform
  # list default is replaced wholesale, not merged: a caller adding one
  # unrelated setting would silently drop TLS enforcement, and the apply would
  # succeed.
  required_parameters = [
    {
      # Refuses any connection not using TLS. The server takes DATABASE_URL
      # verbatim, so this is the only thing enforcing it — hence not
      # overridable.
      name  = "rds.force_ssl"
      value = "1"
      # Static parameter: takes effect on reboot, not on apply. Left "immediate"
      # the plan succeeds and the setting does nothing until the next restart.
      apply_method = "pending-reboot"
    },
  ]

  parameters = concat(local.required_parameters, var.parameters)
}

################################################################################
# Security group
#
# The database's only inbound path. Built here, not in networking/, because it
# is meaningless without the thing it protects. It references the ECS task
# security group by ID — not a CIDR — so only an API task can connect.
################################################################################

resource "aws_security_group" "db" {
  # name_prefix, not name: a replacement can build the new group before
  # destroying the old one, which a fixed name would deadlock on.
  name_prefix = "${var.app_name}-db-"
  description = "PostgreSQL access for ${local.identifier}"
  vpc_id      = var.vpc_id

  lifecycle {
    create_before_destroy = true
  }

  tags = merge(local.tags, { Name = "${var.app_name}-db" })
}

resource "aws_vpc_security_group_ingress_rule" "db" {
  for_each = toset(var.allowed_security_group_ids)

  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = each.value
  from_port                    = var.port
  to_port                      = var.port
  ip_protocol                  = "tcp"
  description                  = "PostgreSQL from ${each.value}"
}

# No egress rules, deliberately. RDS answers connections, it does not open them,
# and Enhanced Monitoring, Performance Insights and IAM auth all travel through
# the RDS service, not this ENI. An extension that calls out needs a rule here.

################################################################################
# The instance
################################################################################

module "db" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 7.2.0"

  identifier = local.identifier

  engine         = "postgres"
  engine_version = var.engine_version
  instance_class = var.instance_class

  # Storage grows on its own up to max_allocated_storage. Without that ceiling
  # autoscaling is off, and a full disk takes the database down.
  allocated_storage     = var.allocated_storage
  max_allocated_storage = var.max_allocated_storage
  storage_type          = var.storage_type
  storage_encrypted     = var.storage_encrypted
  kms_key_id            = var.kms_key_id

  db_name  = var.db_name
  username = var.username
  port     = var.port

  # No password variable anywhere: RDS generates the credential, stores it in
  # Secrets Manager, and hands back an ARN, so it never passes through tfvars, a
  # CI variable, or state. The task definition resolves it by ARN at launch.
  manage_master_user_password   = var.manage_master_user_password
  master_user_secret_kms_key_id = var.master_user_secret_kms_key_id

  ##############################################################################
  # Placement
  ##############################################################################

  # Single-AZ: one instance, one zone, no standby. A zone failure means downtime
  # until AWS restores it or the instance is restored from backup. Multi-AZ
  # roughly doubles the bill to remove that — same trade-off shape as the NAT
  # decision in networking/, so it is explicit rather than inherited.
  multi_az          = var.multi_az
  availability_zone = var.availability_zone

  # The subnet group still spans both AZs even though the instance runs in one:
  # RDS requires two AZs before it will create an instance at all, and it makes
  # converting to Multi-AZ later a one-variable change instead of a rebuild.
  create_db_subnet_group = true
  subnet_ids             = var.private_subnet_ids

  vpc_security_group_ids = [aws_security_group.db.id]

  # Already true by placement — private subnets have no route to the IGW. Set
  # anyway: it is the flag a reviewer looks for, and flipping it undoes that.
  publicly_accessible = false

  ##############################################################################
  # Backups and maintenance
  ##############################################################################

  backup_retention_period = var.backup_retention_period
  backup_window           = var.backup_window
  maintenance_window      = var.maintenance_window
  copy_tags_to_snapshot   = true

  # Refuses to destroy the instance without writing a final snapshot first.
  skip_final_snapshot = var.skip_final_snapshot

  # Minor versions carry security fixes and apply within the maintenance window.
  # Major versions never happen on their own — those change query behavior.
  auto_minor_version_upgrade  = var.auto_minor_version_upgrade
  allow_major_version_upgrade = var.allow_major_version_upgrade

  # Off by default, so changes queue for the maintenance window instead of
  # restarting the database the moment someone runs apply.
  apply_immediately = var.apply_immediately

  ##############################################################################
  # Parameter group
  ##############################################################################

  create_db_parameter_group = true
  family                    = local.family
  parameters                = local.parameters

  # PostgreSQL has no option groups — they are a MySQL/MariaDB/Oracle/SQL
  # Server thing — and the module creates one by default, so this must be off
  # or the apply fails. major_engine_version is omitted for the same reason.
  create_db_option_group = false

  ##############################################################################
  # Observability
  ##############################################################################

  # Ships Postgres logs to CloudWatch for the monitoring layer to alarm on.
  # Without the log group and retention below they would persist forever.
  enabled_cloudwatch_logs_exports        = var.enabled_cloudwatch_logs_exports
  create_cloudwatch_log_group            = true
  cloudwatch_log_group_retention_in_days = var.log_retention_days

  # Free at 7 days retention on every instance class; anything longer bills.
  performance_insights_enabled          = var.performance_insights_enabled
  performance_insights_retention_period = var.performance_insights_retention_period

  # Enhanced Monitoring is OS-level metrics at sub-minute resolution, billed as
  # CloudWatch Logs ingestion. The role is created only when the interval asks
  # for it, so a non-zero interval can never be set without it.
  monitoring_interval    = var.monitoring_interval
  create_monitoring_role = var.monitoring_interval > 0
  monitoring_role_name   = "${local.identifier}-monitoring"

  ##############################################################################
  # Access and protection
  ##############################################################################

  iam_database_authentication_enabled = var.iam_database_authentication_enabled

  # On by default: a delete fails at the API until someone explicitly turns this
  # off, making the destructive step two deliberate actions instead of one.
  # terraform destroy does not get past it either.
  deletion_protection = var.deletion_protection

  tags = local.tags
}
