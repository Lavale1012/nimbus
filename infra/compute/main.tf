# Baseline tags every resource in this module carries, overridable per-caller
# via var.tags. Same shape as networking/local.tags so a cost report can group
# both layers by the same keys instead of two near-identical tag sets.
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

module "ecs" {
  source  = "terraform-aws-modules/ecs/aws"
  version = "~> 6.0"

  cluster_name = local.cluster_name

  # Container Insights bills per metric collected, and this module turns it on
  # by default. Set explicitly so the cost is a decision rather than something
  # inherited from an upstream default.
  cluster_setting = [{
    name  = "containerInsights"
    value = var.container_insights
  }]

  # v6 renamed `fargate_capacity_providers` to this and folded
  # `default_capacity_provider_use_fargate` in with it.
  #
  # `base` is the number of tasks pinned to a provider BEFORE weight applies;
  # weight only splits what is left over. base = 1 keeps one task on capacity
  # AWS cannot reclaim, so a Spot interruption can never take the service to
  # zero. (The registry example's base = 20 sends the first twenty tasks to
  # on-demand — at a two-task scale FARGATE_SPOT would never run at all.)
  default_capacity_provider_strategy = {
    FARGATE      = { base = var.on_demand_base, weight = var.on_demand_weight }
    FARGATE_SPOT = { weight = var.spot_weight }
  }

  services = {
    (local.service_name) = {
      cpu           = var.task_cpu
      memory        = var.task_memory
      desired_count = var.desired_count

      # Placement. Private subnets exported by the networking layer — the whole
      # point of that public/private split is that nothing here has a route in
      # from the internet.
      subnet_ids = var.private_subnet_ids
      vpc_id     = var.vpc_id

      # No public IP: the task is reachable only through the load balancer.
      # Outbound still works — the private subnets route through the NAT gateway
      # networking/ provisions, which is how the image gets pulled from ECR and
      # how the app reaches S3.
      assign_public_ip = false

      # Register with the target group networking/ already built, so the load
      # balancer finally has something to route to. Without this block the ALB
      # has zero targets and every request returns 503.
      load_balancer = {
        alb = {
          target_group_arn = var.target_group_arn
          container_name   = var.container_name # must match the container_definitions key below
          container_port   = var.app_port
        }
      }

      # The module creates a task security group but adds NO rules on its own —
      # both rule maps default to {}. Left empty the ALB could not reach the
      # task, health checks would never pass, and the task could not pull its
      # own image.
      security_group_ingress_rules = {
        alb = {
          from_port   = var.app_port
          to_port     = var.app_port
          ip_protocol = "tcp"
          description = "Application traffic from the load balancer"
          # Referenced by security group ID rather than CIDR, so it stays correct
          # if subnet ranges change and nothing else in the VPC can open a
          # connection to the task.
          referenced_security_group_id = var.alb_security_group_id
        }
      }

      # Outbound has to reach ECR, CloudWatch Logs, and S3, all of which are
      # public endpoints hit through the NAT gateway. VPC endpoints would let
      # this narrow to the VPC the way the ALB's egress does — worth doing once
      # the traffic justifies the per-endpoint hourly cost.
      security_group_egress_rules = {
        all = {
          ip_protocol = "-1"
          cidr_ipv4   = var.task_egress_cidr_ipv4
          description = "Image pulls, log delivery, and S3"
        }
      }

      container_definitions = {
        (var.container_name) = {
          essential = true
          image     = var.container_image

          # Closes the loop on app_port: the target group health-checks it, the
          # port mapping below advertises it, and this makes the server bind it
          # (server.go reads PORT). Without this the variable would be three
          # places agreeing on a number the process ignores.
          environment = concat(
            [{ name = "PORT", value = tostring(var.app_port) }],
            var.container_environment,
          )

          # hostPort is deliberately omitted. Under awsvpc — Fargate's only
          # network mode — each task carries its own ENI and IP, so there is no
          # host port to map onto; AWS requires it to equal containerPort or be
          # left unset.
          portMappings = [{
            name          = var.container_name
            containerPort = var.app_port
            protocol      = "tcp"
          }]

          # This module defaults to true, which is stricter than AWS's own
          # default of false. Worth keeping, but it is a runtime failure rather
          # than a plan error: a binary that writes anything to disk crashes on
          # first write. Flip it only after confirming the app doesn't.
          readonlyRootFilesystem = var.readonly_root_filesystem

          # Shutdown ordering, and the reason this isn't left at the module's
          # default of 120: the server gives in-flight requests 10s
          # (server.go:221) and networking's target group drains for 30s
          # (networking/main.tf:158). A 120s stop timeout holds the container
          # for 90s after both are finished with it, lengthening every deploy.
          stopTimeout = var.stop_timeout

          # Container logs default to never expiring, which is a bill that grows
          # forever for data nobody reads past the incident.
          cloudwatch_log_group_retention_in_days = var.log_retention_days
        }
      }

      # Secrets the execution role may read, so the task definition resolves them
      # by ARN instead of carrying plaintext env vars visible to anyone holding
      # ecs:DescribeTaskDefinition.
      task_exec_secret_arns = var.task_exec_secret_arns

      tags = local.tags
    }
  }

  tags = local.tags
}
