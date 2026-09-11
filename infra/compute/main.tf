# Same tag shape as networking/local.tags so cost reports can group both layers
# by the same keys instead of two near-identical tag sets.
locals {
  tags = merge(
    {
      Terraform   = "true"
      Environment = var.environment
    },
    var.tags,
  )

  cluster_name = coalesce(var.cluster_name, "${var.app_name}-ecs-cluster")
  service_name = coalesce(var.service_name, "${var.app_name}-api")
}

################################################################################
# Cluster and service
################################################################################

module "ecs" {
  source  = "terraform-aws-modules/ecs/aws"
  version = "~> 6.0"

  cluster_name = local.cluster_name

  # Container Insights bills per metric collected and the module turns it on by
  # default. Set explicitly so the cost is a decision, not an inheritance.
  cluster_setting = [{
    name  = "containerInsights"
    value = var.container_insights
  }]

  # v6 renamed `fargate_capacity_providers` to this and folded
  # `default_capacity_provider_use_fargate` in with it.
  #
  # `base` is the task count pinned to a provider BEFORE weight applies; weight
  # splits only what is left over. base = 1 keeps one task on capacity AWS
  # cannot reclaim, so a Spot interruption can never take the service to zero.
  # (The registry example's base = 20 would never let FARGATE_SPOT run here.)
  default_capacity_provider_strategy = {
    FARGATE      = { base = var.on_demand_base, weight = var.on_demand_weight }
    FARGATE_SPOT = { weight = var.spot_weight }
  }

  services = {
    (local.service_name) = {
      cpu           = var.task_cpu
      memory        = var.task_memory
      desired_count = var.desired_count

      ##########################################################################
      # Placement
      ##########################################################################

      # Private subnets from the networking layer — the point of that split is
      # that nothing here has a route in from the internet.
      subnet_ids = var.private_subnet_ids
      vpc_id     = var.vpc_id

      # No public IP: reachable only through the load balancer. Outbound still
      # works via networking/'s NAT gateway, which is how the image is pulled
      # from ECR and how the app reaches S3.
      assign_public_ip = false

      # Registers with the target group networking/ already built. Without this
      # block the ALB has zero targets and every request returns 503.
      load_balancer = {
        alb = {
          target_group_arn = var.target_group_arn
          # Must match the container_definitions key below.
          container_name = var.container_name
          container_port = var.app_port
        }
      }

      ##########################################################################
      # Security group
      ##########################################################################

      # The module creates a task security group but adds NO rules on its own,
      # both rule maps defaulting to {}. Left empty, the ALB cannot reach the
      # task, health checks never pass, and the task cannot pull its own image.
      security_group_ingress_rules = {
        alb = {
          from_port   = var.app_port
          to_port     = var.app_port
          ip_protocol = "tcp"
          description = "Application traffic from the load balancer"
          # By security group ID, not CIDR: stays correct if subnet ranges
          # change, and nothing else in the VPC can reach the task.
          referenced_security_group_id = var.alb_security_group_id
        }
      }

      # Outbound must reach ECR, CloudWatch Logs and S3 — public endpoints hit
      # through the NAT gateway. VPC endpoints would narrow this to the VPC,
      # worth doing once traffic justifies the per-endpoint hourly cost.
      security_group_egress_rules = {
        all = {
          ip_protocol = "-1"
          cidr_ipv4   = var.task_egress_cidr_ipv4
          description = "Image pulls, log delivery, and S3"
        }
      }

      ##########################################################################
      # Container definition
      ##########################################################################

      container_definitions = {
        (var.container_name) = {
          essential = true
          image     = var.container_image

          # Closes the loop on app_port: the target group health-checks it, the
          # mapping below advertises it, and this makes the server actually bind
          # it (server.go reads PORT).
          environment = concat(
            [{ name = "PORT", value = tostring(var.app_port) }],
            var.container_environment,
          )

          # hostPort deliberately omitted: under awsvpc — Fargate's only
          # network mode — each task has its own ENI and IP, so there is no
          # host port to map onto. AWS requires it to equal containerPort
          # or be left unset.
          portMappings = [{
            name          = var.container_name
            containerPort = var.app_port
            protocol      = "tcp"
          }]

          # The module defaults to true, stricter than AWS's own false. Worth
          # keeping, but it fails at runtime rather than plan: a binary that
          # writes to disk crashes on first write.
          readonlyRootFilesystem = var.readonly_root_filesystem

          # Not the module's default of 120: the server gives in-flight requests
          # 10s (server.go:221) and the target group drains for 30s
          # (networking/main.tf:158), so 120 would hold the container 90s past
          # the point both are done with it, lengthening every deploy.
          stopTimeout = var.stop_timeout

          # Container logs default to never expiring — a bill that grows
          # forever for data nobody reads past the incident.
          cloudwatch_log_group_retention_in_days = var.log_retention_days
        }
      }

      # Secrets the execution role may read, so the task definition resolves
      # them by ARN rather than carrying plaintext env vars that anyone with
      # ecs:DescribeTaskDefinition can read.
      task_exec_secret_arns = var.task_exec_secret_arns

      tags = local.tags
    }
  }

  tags = local.tags
}
