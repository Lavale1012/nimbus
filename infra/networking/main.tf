# Network foundation: VPC, subnets across two AZs, and the gateways that
# connect them to the internet.
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"

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

  # Baseline tags, overridable per-caller via var.tags.
  tags = merge(
    {
      Terraform   = "true"
      Environment = var.environment
    },
    var.tags,
  )
}

# Public-facing load balancer. Terminates TLS and forwards to ECS tasks.
module "alb" {
  source = "terraform-aws-modules/alb/aws"

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

  # Bucket must already exist and grant write access to the ELB service.
  access_logs = {
    bucket = "${var.app_name}-alb-logs"
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

  tags = {
    Environment = "Development"
    Project     = "Example"
  }
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
