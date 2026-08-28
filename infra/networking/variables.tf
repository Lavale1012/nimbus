variable "app_name" {
  type        = string
  description = "name of app name."
}

variable "environment" {
  type        = string
  description = "Deployment environment the VPC belongs to (e.g. dev, staging, prod)."
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR block for the VPC."
}

variable "azs" {
  type        = list(string)
  description = "Availability zones to spread the subnets across."
}

variable "private_subnets" {
  type        = list(string)
  description = "CIDR blocks for the private subnets, one per availability zone."
}

variable "public_subnets" {
  type        = list(string)
  description = "CIDR blocks for the public subnets, one per availability zone."
}

variable "enable_nat_gateway" {
  type        = bool
  description = "Provision NAT gateways so private subnets can reach the internet."
}

variable "single_nat_gateway" {
  type        = bool
  description = "Share a single NAT gateway across all AZs instead of one per AZ."
}

variable "one_nat_gateway_per_az" {
  type        = bool
  default     = false
  description = "Place one NAT gateway in each AZ's public subnet, so a zone's outbound traffic never crosses zones. Requires at least as many public subnets as AZs. Has no effect when single_nat_gateway is true."

  validation {
    condition     = !(var.single_nat_gateway && var.one_nat_gateway_per_az)
    error_message = "single_nat_gateway and one_nat_gateway_per_az are mutually exclusive; single_nat_gateway wins silently, so set it to false to get one NAT per AZ."
  }
  # Also guards against paying for per-AZ NAT gateways that were never created.
}

variable "enable_vpn_gateway" {
  type        = bool
  description = "Provision a VPN gateway for the VPC."
}

variable "tags" {
  type        = map(string)
  description = "Additional tags applied to all resources created by the module."
}

variable "igw" {
  type        = bool
  description = "Internet gateway"
}

variable "alb_ingress_cidr_ipv4" {
  type        = string
  description = "IPv4 CIDR allowed to reach the load balancer on HTTP and HTTPS. Use 0.0.0.0/0 for a public ALB."
}

variable "alb_egress_cidr_ipv4" {
  type        = string
  description = "IPv4 CIDR the load balancer may send traffic to. Normally the VPC CIDR, so the ALB can only reach its targets."
}

variable "http_port" {
  type        = number
  default     = 80
  description = "Port the load balancer accepts plain HTTP on, before redirecting to HTTPS."
}

variable "https_port" {
  type        = number
  default     = 443
  description = "Port the load balancer terminates TLS on."
}

variable "alb_log_retention_days" {
  type        = number
  default     = 90
  description = "Days to keep ALB access logs before expiring them. Logs accumulate forever otherwise."

  validation {
    condition     = var.alb_log_retention_days > 0
    error_message = "alb_log_retention_days must be greater than zero."
  }
}

variable "alb_logs_force_destroy" {
  type        = bool
  default     = false
  description = "Allow the access log bucket to be destroyed while it still holds objects. Leave false outside throwaway environments; a terraform destroy will otherwise fail on the non-empty bucket."
}

variable "domain_name" {
  type        = string
  description = "Domain name for the ACM certificate, and the name pointed at the load balancer."
}

variable "hosted_zone_name" {
  type        = string
  description = "Route53 public hosted zone holding the DNS validation records, e.g. nimbuscli.us."
}

variable "app_port" {
  type        = number
  description = "Port the API server listens on, used as the target group port."
}

variable "target_group_name_prefix" {
  type        = string
  description = "Prefix AWS builds the target group name from. Six characters maximum."

  validation {
    condition     = length(var.target_group_name_prefix) <= 6
    error_message = "Target group name_prefix must be 6 characters or fewer."
  }
  # AWS rejects a longer prefix at apply time, so catch it during plan.
}

variable "health_check" {
  type = object({
    path                = string
    matcher             = optional(string, "200")
    interval            = optional(number, 30)
    timeout             = optional(number, 5)
    healthy_threshold   = optional(number, 2)
    unhealthy_threshold = optional(number, 2)
  })
  description = "Target group health check. Only path is required; the rest fall back to AWS-typical values."
}
