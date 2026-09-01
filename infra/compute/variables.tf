# Same principle as networking/variables.tf: environment-shaping choices carry
# no default, so the decision lives in the caller's config where a human reviews
# it. Defaults appear only where the value is a safe floor or a fixed constant.

variable "app_name" {
  type        = string
  description = "Name of the application. Cluster and service names derive from it unless overridden below."
}

variable "environment" {
  type        = string
  description = "Deployment environment the cluster belongs to (e.g. dev, staging, prod). Becomes the Environment tag."
}

variable "tags" {
  type        = map(string)
  description = "Additional tags applied to all resources created by the module, merged over the Terraform/Environment baseline."
}

################################################################################
# From the networking layer
#
# These four are exactly the outputs networking/outputs.tf exists to provide.
# Passing them as variables rather than reading remote state keeps this module
# testable on its own and makes the dependency explicit at the call site.
################################################################################

variable "vpc_id" {
  type        = string
  description = "VPC the tasks run in. Used for the task security group."
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs to place tasks in. These must have no route to the internet gateway; that absence is what keeps tasks unreachable from outside."

  validation {
    condition     = length(var.private_subnet_ids) >= 2
    error_message = "private_subnet_ids needs at least two subnets in different AZs; a single subnet means a zone failure takes the whole service down."
  }
}

variable "alb_security_group_id" {
  type        = string
  description = "Security group attached to the load balancer. The task security group allows ingress from this ID rather than a CIDR, so it stays correct if subnet ranges change."
}

variable "target_group_arn" {
  type        = string
  description = "ARN of the target group the service registers task IPs against. Without it the load balancer has no targets."
}

################################################################################
# Naming
################################################################################

variable "cluster_name" {
  type        = string
  default     = null
  description = "Explicit ECS cluster name. Leave null to derive it as <app_name>-ecs-cluster; set it only to match a cluster that already exists, since renaming forces a replacement."
}

variable "service_name" {
  type        = string
  default     = null
  description = "Explicit ECS service name. Leave null to derive it as <app_name>-api."
}

variable "container_name" {
  type        = string
  description = "Name of the application container. Must be referenced identically by the load balancer block, which is why it is a single variable rather than two literals."
}

################################################################################
# Task definition
################################################################################

variable "container_image" {
  type        = string
  description = "Full image URI the task runs, e.g. <account>.dkr.ecr.<region>.amazonaws.com/nimbus-api:<tag>. No default: an image is not something a module should guess."
}

variable "app_port" {
  type        = number
  description = "Port the API server listens on. Must match the target group port in networking/ and the port the server actually binds."
}

variable "task_cpu" {
  type        = number
  description = "CPU units for the task. Fargate accepts only specific CPU/memory pairs; 512 units is 0.5 vCPU."
}

variable "task_memory" {
  type        = number
  description = "Memory (MiB) for the task. Must be a value Fargate permits for the chosen task_cpu, or the task definition is rejected at apply time."
}

variable "desired_count" {
  type        = number
  description = "Number of tasks to keep running. Two or more, one per AZ, is what makes a zone failure survivable and lets ECS replace one task at a time behind the load balancer."

  validation {
    condition     = var.desired_count >= 1
    error_message = "desired_count must be at least 1."
  }
}

variable "stop_timeout" {
  type        = number
  default     = 20
  description = "Seconds ECS waits after SIGTERM before SIGKILL. Must sit above the server's 10s graceful shutdown and below the target group's 30s deregistration_delay in networking/. The upstream module defaults this to 120, which strands a container for 90s after both the app and the load balancer are done with it."

  validation {
    condition     = var.stop_timeout > 10 && var.stop_timeout <= 120
    error_message = "stop_timeout must exceed the server's 10s graceful shutdown so in-flight requests finish, and cannot exceed 120 — the Fargate ceiling. Keep it under networking's deregistration_delay (30) as well."
  }
}

variable "readonly_root_filesystem" {
  type        = bool
  default     = true
  description = "Give the container a read-only root filesystem. The upstream module defaults this to true, stricter than AWS's own default. Set false only after confirming the app writes nothing to disk, because the failure is a runtime crash rather than a plan error."
}

variable "log_retention_days" {
  type        = number
  default     = 14
  description = "Days to retain container logs. Logs default to never expiring, which is a bill that grows forever for data nobody reads past the incident. Matches networking's alb_log_retention_days so an incident spanning both layers has request logs and application logs covering the same window."

  validation {
    condition     = var.log_retention_days > 0
    error_message = "log_retention_days must be greater than zero; 0 means retain indefinitely, which is the outcome this variable exists to prevent."
  }
}

variable "container_environment" {
  type = list(object({
    name  = string
    value = string
  }))
  default     = []
  description = "Additional plaintext environment variables for the container. PORT is always injected from app_port and does not belong here. Anything secret belongs in task_exec_secret_arns instead — values here are readable by anyone holding ecs:DescribeTaskDefinition."

  validation {
    condition     = !contains([for e in var.container_environment : e.name], "PORT")
    error_message = "PORT is derived from app_port and injected automatically; setting it here would produce a duplicate environment entry and let the server bind a port the target group does not health-check."
  }
}

variable "task_exec_secret_arns" {
  type        = list(string)
  default     = []
  description = "Secrets Manager ARNs the execution role may read, so the task definition resolves secrets by ARN instead of carrying them as plaintext environment variables."
}

################################################################################
# Cluster settings
################################################################################

variable "container_insights" {
  type        = string
  default     = "disabled"
  description = "Container Insights mode for the cluster: disabled, enabled, or enhanced. Billed per metric collected, and the upstream module enables it by default, so it is pinned here."

  validation {
    condition     = contains(["disabled", "enabled", "enhanced"], var.container_insights)
    error_message = "container_insights must be one of: disabled, enabled, enhanced. ECS accepts no other value and rejects it at apply time."
  }
}

variable "task_egress_cidr_ipv4" {
  type        = string
  default     = "0.0.0.0/0"
  description = "IPv4 CIDR tasks may send traffic to. Wide by necessity: ECR, CloudWatch Logs, and S3 are public endpoints reached through the NAT gateway. VPC endpoints would let this narrow to the VPC CIDR."
}

################################################################################
# Capacity providers
#
# base pins a number of tasks to a provider before weight applies; weight splits
# only what remains. Getting these backwards is why the registry example's
# 50/50 split silently runs everything on-demand.
################################################################################

variable "on_demand_base" {
  type        = number
  default     = 1
  description = "Tasks pinned to on-demand FARGATE before weights apply. At least 1 means a Spot reclamation can never take the service to zero."
}

variable "on_demand_weight" {
  type        = number
  default     = 50
  description = "Relative share of tasks beyond on_demand_base placed on on-demand FARGATE."
}

variable "spot_weight" {
  type        = number
  default     = 50
  description = "Relative share of tasks beyond on_demand_base placed on FARGATE_SPOT. Spot is roughly 70% cheaper but AWS can reclaim a task with two minutes' notice. Set to 0 to run entirely on-demand."
}
