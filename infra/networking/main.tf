# Baseline tags every resource in this module carries, overridable per-caller
# via var.tags. Defined once so the ALB and the log bucket can't drift from
# the VPC the way hardcoded tags did.
locals {
  tags = merge(
    {
      Terraform   = "true"
      Environment = var.environment
    },
    var.tags,
  )
}

# Network foundation: VPC, subnets across two AZs, and the gateways that
# connect them to the internet.
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.7.0"

  name = var.app_name
  cidr = var.vpc_cidr

  # Subnet lists are zipped against azs by position, so all three must be
  # the same length. Public subnets host the ALB, private ones the ECS tasks.
  azs             = var.azs
  private_subnets = var.private_subnets
  public_subnets  = var.public_subnets

  enable_nat_gateway = var.enable_nat_gateway # outbound internet for private subnets
  # One shared NAT, landing in the first public subnet (azs[0]). Every private
  # route table points at it, so that AZ going down cuts outbound for both.
  single_nat_gateway = var.single_nat_gateway
  # Set instead of single_nat_gateway to get a NAT per AZ; the two conflict.
  one_nat_gateway_per_az = var.one_nat_gateway_per_az
  enable_vpn_gateway     = var.enable_vpn_gateway
  create_igw             = var.igw # inbound internet for public subnets

  tags = local.tags
}

# Destination for ALB access logs. The load balancer cannot be created until
# this bucket exists AND carries a policy letting the log delivery service
# write to it — AWS verifies delivery up front and fails the create otherwise.
module "alb_logs" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 5.15.0"

  bucket = "${var.app_name}-alb-logs"

  # Writes the log-delivery policy for ALB/NLB. The correct principal differs
  # by region age, so this is left to the module rather than hand-rolled.
  attach_lb_log_delivery_policy = true

  # The module blocks all public access by default, so that is left alone.
  force_destroy = var.alb_logs_force_destroy

  # ALB access logs support SSE-S3 only, not a customer-managed KMS key.
  server_side_encryption_configuration = {
    rule = {
      apply_server_side_encryption_by_default = {
        sse_algorithm = "AES256"
      }
    }
  }

  # Access logs accumulate forever by default; this is a bill that grows for
  # data nobody reads past the incident it was needed for.
  lifecycle_rule = [
    {
      id      = "expire-access-logs"
      enabled = true
      expiration = {
        days = var.alb_log_retention_days
      }
    }
  ]

  tags = local.tags
}

# Public-facing load balancer. Terminates TLS and forwards to ECS tasks.
module "alb" {
  source  = "terraform-aws-modules/alb/aws"
  version = "~> 10.5.0"

  # s3_bucket_id resolves from the bucket, not the bucket policy, so referencing
  # it alone would let Terraform build the ALB before the policy is attached and
  # fail intermittently. Depending on the whole module waits for both.
  depends_on = [module.alb_logs]

  name    = "${var.app_name}-alb"
  vpc_id  = module.vpc.vpc_id
  subnets = module.vpc.public_subnets # internet-reachable

  # Security Group
  # Each map entry becomes one SG rule. Keys are Terraform labels only;
  # renaming one destroys and recreates that rule.
  security_group_ingress_rules = {
    all_http = {
      from_port   = var.http_port
      to_port     = var.http_port
      ip_protocol = "tcp"
      description = "HTTP web traffic"
      cidr_ipv4   = var.alb_ingress_cidr_ipv4
    }
    all_https = {
      from_port   = var.https_port
      to_port     = var.https_port
      ip_protocol = "tcp"
      description = "HTTPS web traffic"
      cidr_ipv4   = var.alb_ingress_cidr_ipv4
    }
  }
  # Narrows the AWS default of all-outbound-anywhere: "-1" is every protocol,
  # but only to the VPC, so the ALB can reach its targets and nothing else.
  security_group_egress_rules = {
    all = {
      ip_protocol = "-1"
      cidr_ipv4   = var.alb_egress_cidr_ipv4
    }
  }

  access_logs = {
    enabled = true
    bucket  = module.alb_logs.s3_bucket_id
  }

  listeners = {
    # Port 80 never reaches the app; it just bounces clients to HTTPS.
    http-https-redirect = {
      port     = var.http_port
      protocol = "HTTP"
      redirect = {
        port        = tostring(var.https_port) # redirect target is a string, unlike port above
        protocol    = "HTTPS"
        status_code = "HTTP_301" # permanent, so browsers stop retrying the HTTP port
      }
    }
    # TLS terminates here, then plaintext to the target group.
    https = {
      port            = var.https_port
      protocol        = "HTTPS"
      certificate_arn = aws_acm_certificate_validation.this.certificate_arn

      forward = {
        target_group_key = "ecs_task" # must match the target_groups key below
      }
    }
  }

  target_groups = {
    ecs_task = {
      # AWS appends a random suffix; the prefix is capped at 6 characters.
      name_prefix                       = var.target_group_name_prefix
      protocol                          = "HTTP" # ALB to task, inside the VPC
      port                              = var.app_port
      target_type                       = "ip" # ECS registers task IPs, not instance IDs
      deregistration_delay              = 30   # seconds to drain before removing a task
      load_balancing_cross_zone_enabled = true # spread traffic across both AZs

      # Failing targets get pulled from rotation, so path must return 200.
      health_check = {
        enabled             = true
        protocol            = "HTTP"
        port                = "traffic-port" # follows the target group port above
        path                = var.health_check.path
        matcher             = var.health_check.matcher
        interval            = var.health_check.interval
        timeout             = var.health_check.timeout
        healthy_threshold   = var.health_check.healthy_threshold
        unhealthy_threshold = var.health_check.unhealthy_threshold
      }
    }
  }

  tags = local.tags
}

# TLS certificate for the HTTPS listener. Issued in PENDING_VALIDATION and
# stays unusable until the DNS records below prove domain ownership.
resource "aws_acm_certificate" "this" {
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true # replace before dropping the in-use cert
  }
}

# Existing public zone that hosts the validation records.
data "aws_route53_zone" "this" {
  name         = var.hosted_zone_name
  private_zone = false
}

# One CNAME per domain on the cert, proving ownership to ACM.
resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.this.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  zone_id         = data.aws_route53_zone.this.zone_id
  name            = each.value.name
  type            = each.value.type
  records         = [each.value.record]
  ttl             = 60
  allow_overwrite = true # tolerate leftover records from a previous run
}

# Blocks until ACM marks the cert ISSUED. The listener depends on this rather
# than the certificate itself, so it never attaches an unvalidated cert.
resource "aws_acm_certificate_validation" "this" {
  certificate_arn         = aws_acm_certificate.this.arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]
}

# Points the domain at the load balancer. The certificate above only proves
# ownership of the name; without this record the name resolves nowhere.
#
# Deliberately a second, separate call rather than folding the validation
# records above into this one: those records gate the certificate, the
# certificate gates the HTTPS listener, and this record reads the ALB those
# listeners belong to. One module holding both halves would close that chain
# into a dependency cycle Terraform refuses to plan.
module "dns_alias" {
  source  = "terraform-aws-modules/route53/aws"
  version = "~> 6.5.0"

  # Look up the existing zone instead of creating one; the domain predates
  # this infrastructure and a destroy here must not be able to take it down.
  create_zone = false
  name        = var.hosted_zone_name

  records = {
    alb = {
      # full_name is used verbatim. Without it the map key would be treated as
      # a subdomain prefix, which breaks when domain_name is the zone apex.
      full_name = var.domain_name
      type      = "A"

      # An alias, not a CNAME: a zone apex cannot hold a CNAME, and Route53
      # answers alias queries for free.
      alias = {
        name    = module.alb.dns_name
        zone_id = module.alb.zone_id
        # Pulls the record when every target is failing health checks.
        evaluate_target_health = true
      }
    }
  }

  tags = local.tags
}
