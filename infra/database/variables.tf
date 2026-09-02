# Same principle as networking/variables.tf and compute/variables.tf:
# environment-shaping choices carry no default, so the decision lives in the
# caller's config where a human reviews it. Defaults appear only where the value
# is a safe floor, a fixed constant, or the protective option.

variable "app_name" {
  type        = string
  description = "Name of the application. The instance identifier and security group name derive from it unless overridden below."
}

variable "environment" {
  type        = string
  description = "Deployment environment the database belongs to (e.g. dev, staging, prod). Becomes the Environment tag."
}

variable "tags" {
  type        = map(string)
  description = "Additional tags applied to all resources created by the module, merged over the Terraform/Environment baseline."
}

################################################################################
# From the networking layer
#
# Passed as variables rather than read from remote state, so this module stays
# testable on its own and the dependency is explicit at the call site.
################################################################################

variable "vpc_id" {
  type        = string
  description = "VPC the database runs in. Used for the database security group."
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs for the DB subnet group. These must have no route to the internet gateway; that absence is what keeps the database unreachable from outside."

  validation {
    condition     = length(var.private_subnet_ids) >= 2
    error_message = "RDS requires a DB subnet group covering at least two availability zones, even for a single-AZ instance. Pass both private subnets."
  }
}

################################################################################
# From the compute layer
################################################################################

variable "allowed_security_group_ids" {
  type        = list(string)
  description = "Security groups permitted to open connections to the database, normally just the ECS task security group. Referenced by ID rather than CIDR so the rule stays correct if subnet ranges change."

  validation {
    condition     = length(var.allowed_security_group_ids) > 0
    error_message = "A database nothing can connect to is almost certainly a mistake; pass the ECS task security group id."
  }
}

################################################################################
# Naming
################################################################################

variable "identifier" {
  type        = string
  default     = null
  description = "Explicit RDS instance identifier. Leave null to derive it as <app_name>-db; set it only to match an instance that already exists, since renaming forces a replacement."
}

variable "db_name" {
  type        = string
  description = "Name of the database created inside the instance. This is the database the application connects to, not the instance identifier."
}

variable "username" {
  type        = string
  description = "Master username. Cannot be 'postgres', 'admin', 'rdsadmin' or other reserved names."

  validation {
    condition     = !contains(["postgres", "admin", "rdsadmin", "root"], lower(var.username))
    error_message = "RDS reserves this username and rejects it at apply time; pick another."
  }
}

variable "port" {
  type        = number
  default     = 5432
  description = "Port PostgreSQL listens on. Also the port opened in the security group."
}

################################################################################
# Engine and sizing
################################################################################

variable "engine_version" {
  type        = string
  default     = "17"
  description = "PostgreSQL version. A major-only value such as \"17\" lets AWS select the current minor within that major, which is what auto_minor_version_upgrade then keeps current."
}

variable "family" {
  type        = string
  default     = null
  description = "Parameter group family. Leave null to derive it from engine_version as postgres<major>; set it only when the derived value is wrong."
}

variable "instance_class" {
  type        = string
  description = "Instance class, e.g. db.t4g.micro. The largest recurring cost in this module, so it carries no default."
}

variable "allocated_storage" {
  type        = number
  description = "Initial storage in GiB. RDS enforces a floor of 20 for gp2/gp3."

  validation {
    condition     = var.allocated_storage >= 20
    error_message = "RDS rejects allocated_storage below 20 GiB for gp2/gp3 at apply time."
  }
}

variable "max_allocated_storage" {
  type        = number
  description = "Ceiling in GiB that storage autoscaling may grow to. Must exceed allocated_storage; autoscaling is disabled when they are equal, which turns a full disk into an outage."

  validation {
    condition     = var.max_allocated_storage > var.allocated_storage
    error_message = "max_allocated_storage must be greater than allocated_storage, otherwise storage autoscaling never engages."
  }
}

variable "storage_type" {
  type        = string
  default     = "gp3"
  description = "EBS volume type backing the instance."
}

variable "storage_encrypted" {
  type        = bool
  default     = true
  description = "Encrypt storage at rest. Cannot be changed in place — turning this on later means a snapshot, restore, and cutover."
}

variable "kms_key_id" {
  type        = string
  default     = null
  description = "Customer-managed KMS key for storage encryption. Null uses the AWS-managed RDS key, which is free but cannot be shared across accounts or rotated on your schedule."
}

################################################################################
# Placement
################################################################################

variable "multi_az" {
  type        = bool
  default     = false
  description = "Run a synchronous standby in a second AZ with automatic failover. Roughly doubles the instance bill. False means a zone failure is downtime until AWS restores the zone or the instance is restored from a snapshot."
}

variable "availability_zone" {
  type        = string
  default     = null
  description = "Pin the single-AZ instance to a specific zone, e.g. us-east-1a. Null lets RDS choose. Ignored — and rejected — when multi_az is true, since a Multi-AZ instance spans two zones by definition."

  validation {
    condition     = !(var.multi_az && var.availability_zone != null)
    error_message = "availability_zone cannot be set while multi_az is true; AWS rejects the combination. Leave it null for Multi-AZ."
  }
}

################################################################################
# Credentials
################################################################################

variable "manage_master_user_password" {
  type        = bool
  default     = true
  description = "Have RDS generate the master password and store it in Secrets Manager. Leaving this on is why no password variable exists here: the credential never passes through tfvars, CI, or the state file."
}

variable "master_user_secret_kms_key_id" {
  type        = string
  default     = null
  description = "Customer-managed KMS key encrypting the generated master password secret. Null uses the AWS-managed key."
}

variable "iam_database_authentication_enabled" {
  type        = bool
  default     = true
  description = "Allow IAM credentials to authenticate to the database, so a task can connect with its role instead of a long-lived password."
}

################################################################################
# Backups and maintenance
################################################################################

variable "backup_retention_period" {
  type        = number
  default     = 7
  description = "Days of automated backups to keep. Zero disables backups entirely and also disables point-in-time recovery."

  validation {
    condition     = var.backup_retention_period > 0
    error_message = "backup_retention_period of 0 disables automated backups and point-in-time recovery; set at least 1."
  }
}

variable "backup_window" {
  type        = string
  default     = "03:00-06:00"
  description = "UTC window for automated backups. Must not overlap maintenance_window."
}

variable "maintenance_window" {
  type        = string
  default     = "Mon:00:00-Mon:03:00"
  description = "UTC window for patching and minor version upgrades. Must not overlap backup_window."
}

variable "skip_final_snapshot" {
  type        = bool
  default     = false
  description = "Destroy the instance without writing a final snapshot. Leave false outside throwaway environments; true means a destroy is unrecoverable."
}

variable "auto_minor_version_upgrade" {
  type        = bool
  default     = true
  description = "Apply minor version upgrades during the maintenance window. These carry security fixes and do not change query behavior."
}

variable "allow_major_version_upgrade" {
  type        = bool
  default     = false
  description = "Permit major version upgrades. Off by default because majors can change query planning and break the application; flip it deliberately for a planned upgrade."
}

variable "apply_immediately" {
  type        = bool
  default     = false
  description = "Apply modifications at once instead of queuing them for the maintenance window. True can restart the database in the middle of a workday."
}

################################################################################
# Parameters
################################################################################

variable "parameters" {
  type = list(object({
    name         = string
    value        = string
    apply_method = optional(string)
  }))

  default = [
    {
      # Refuses any connection that is not using TLS. The application pays a
      # connection parameter; the alternative is credentials and file metadata
      # crossing the VPC in the clear.
      name  = "rds.force_ssl"
      value = "1"
      # Static parameter: it takes effect on reboot, not on apply. Left as
      # "immediate" the plan succeeds and the setting silently does nothing
      # until the next restart.
      apply_method = "pending-reboot"
    },
  ]

  description = "Parameter group entries. Defaults to requiring TLS on every connection."
}

################################################################################
# Observability
################################################################################

variable "enabled_cloudwatch_logs_exports" {
  type        = list(string)
  default     = ["postgresql", "upgrade"]
  description = "Log types shipped to CloudWatch. postgresql carries query errors and connection failures; upgrade carries major version upgrade output."
}

variable "log_retention_days" {
  type        = number
  default     = 14
  description = "Days to keep exported database logs. Matches compute's log_retention_days so an incident spanning both layers has application and database logs covering the same window."

  validation {
    condition     = var.log_retention_days > 0
    error_message = "log_retention_days must be greater than zero."
  }
}

variable "performance_insights_enabled" {
  type        = bool
  default     = true
  description = "Capture query-level performance data. Free at the 7-day retention below."
}

variable "performance_insights_retention_period" {
  type        = number
  default     = 7
  description = "Days of Performance Insights history. 7 is the free tier; anything longer bills per vCPU per month."
}

variable "monitoring_interval" {
  type        = number
  default     = 0
  description = "Seconds between Enhanced Monitoring samples of OS-level metrics. 0 disables it. Valid values are 0, 1, 5, 10, 15, 30 and 60; non-zero bills as CloudWatch Logs ingestion, which is why this is opt-in."

  validation {
    condition     = contains([0, 1, 5, 10, 15, 30, 60], var.monitoring_interval)
    error_message = "monitoring_interval must be one of 0, 1, 5, 10, 15, 30, 60."
  }
}

################################################################################
# Protection
################################################################################

variable "deletion_protection" {
  type        = bool
  default     = true
  description = "Refuse deletion of the instance at the AWS API. On by default: the destructive step becomes two deliberate actions instead of one. A terraform destroy fails until this is turned off and applied."
}
