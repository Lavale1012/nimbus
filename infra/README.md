# Nimbus Infrastructure — A Guided Tour

The AWS infrastructure behind Nimbus, written as code with Terraform. One section per
layer, explaining what it builds, why I built it that way, and what the choice buys.

| § | Layer | Status |
| --- | --- | --- |
| [1](#1--networking) | **Networking** — VPC, subnets, NAT, ALB, TLS | **Built** |
| [2](#2--compute) | **Compute** — ECS Fargate, ECR, task definition | Planned |
| [3](#3--database) | **Database** — RDS PostgreSQL | Planned |
| [4](#4--monitoring) | **Monitoring** — CloudWatch, SNS | Planned |
| [5](#5--cross-cutting) | **Cross-cutting** — state, tagging, secrets | Partly built |

Only Networking is built today. The rest are documented with the decisions already made
and what they attach to — I'd rather ship a deliberate foundation than a broad `apply`
that works by accident.

---

## The problem this solves

Nimbus is cloud file storage: a Go CLI talks to a Go API, which stores metadata in
PostgreSQL and hands back short-lived S3 URLs so file bytes go straight from your machine
to S3. That's the application. This directory answers a different question — **how does a
request reach that API safely, and how does the API reach the internet without being
exposed to it?**

```text
  laptop (nim CLI)
       │  HTTPS :443
       ▼
  PUBLIC SUBNETS (AZ-a, AZ-b) ─────────────────── NETWORKING
    Application Load Balancer  ← TLS terminates here
    NAT Gateway
       │  plain HTTP :8080, inside the VPC
       ▼
  PRIVATE SUBNETS (AZ-a, AZ-b) ─────────────────── COMPUTE
    ECS Fargate tasks — no public IP
       │  :5432, security-group-locked
       ▼
  PRIVATE SUBNETS ──────────────────────────────── DATABASE
    RDS PostgreSQL — reachable only from the tasks
```

---

## 1 · Networking

**Built.** [`networking/`](networking/) — ~170 lines: `main.tf`, `variables.tf`,
`outputs.tf`.

Almost no variable has a default. Defaults in a shared module are invisible decisions — if
`vpc_cidr` quietly defaults to `10.0.0.0/16`, a second environment gets an overlapping
network and you find out during a peering attempt six months later. Requiring them moves
the choice into the caller's config where a human reviews it.

### 1.1 · The VPC

[`main.tf:3-32`](networking/main.tf#L3-L32) — one VPC, public and private subnets,
mirrored across **two AZs**.

An AZ is a physically separate datacenter, so two is the smallest number that survives
losing one. The public/private split is the real security boundary: public subnets have a
route to the Internet Gateway, private ones don't — so a container there **cannot** be
reached from the internet no matter how it's misconfigured. Not a firewall rule someone
can loosen; the absence of a road. Tasks go private, only the ALB goes public.

**Sharp edge:** `azs`, `private_subnets`, and `public_subnets` are zipped *by position*,
and nothing enforces equal length. Get it wrong and you silently deploy an unbalanced
network — hence the comment at [`main.tf:9-13`](networking/main.tf#L9-L13).

### 1.2 · NAT gateways

Private subnets can't be reached from the internet, but the API still needs to *reach
out* — pull images, call S3. A NAT gateway is that one-way valve, and there are two ways
to run it:

| | One shared NAT | One per AZ |
| --- | --- | --- |
| Cost | ~$32/mo | ~$32/mo **per AZ** |
| If that AZ fails | **both** AZs lose outbound | only that AZ |

I support both and default to shared — doubling the bill for a rare AZ failure isn't right
at this scale, but production might decide otherwise.

The trap: the upstream VPC module doesn't error when you set *both*. It silently ignores
`one_nat_gateway_per_az`. You'd write config asking for HA, get a clean plan, apply
successfully, and believe you had redundancy that doesn't exist — a bug that only surfaces
during the failure you were trying to survive. So
[`variables.tf:41-51`](networking/variables.tf#L41-L51):

```hcl
validation {
  condition     = !(var.single_nat_gateway && var.one_nat_gateway_per_az)
  error_message = "single_nat_gateway and one_nat_gateway_per_az are mutually exclusive; single_nat_gateway wins silently, so set it to false to get one NAT per AZ."
}
```

A silent, expensive misconfiguration became a loud plan-time error whose message names the
fix.

### 1.3 · The load balancer

[`main.tf:35-73`](networking/main.tf#L35-L73) — inbound on 80 and 443 from a
caller-supplied CIDR (`0.0.0.0/0` for public, an office range for staging), and — the part
people skip — **narrowed outbound**:

```hcl
security_group_egress_rules = {
  all = {
    ip_protocol = "-1"                        # every protocol...
    cidr_ipv4   = var.alb_egress_cidr_ipv4    # ...but only to the VPC
  }
}
```

AWS security groups default to allow-all-outbound, and almost nobody changes it because
outbound feels harmless. It isn't: unrestricted egress is how data leaves and how a
foothold phones home. The ALB's job is forwarding to targets inside the VPC, so anything
else it tries to reach is by definition not its job.

**Terraform detail** ([`main.tf:43-44`](networking/main.tf#L43-L44)): the `all_http` /
`all_https` keys are labels, but Terraform tracks resources by them. Renaming one
*destroys* a rule and *creates* another — a brief gap on a live system.

### 1.4 · Listeners

[`main.tf:75-96`](networking/main.tf#L75-L96) — port 80 exists purely to redirect and
never forwards to the app. You can't just close it (a bare hostname would refuse the
connection), so it stays open with nothing to do but send you to HTTPS.

The redirect is a **301**, not a 302: a 302 makes clients retry port 80 *every time*, while
a 301 is cached and they go straight to HTTPS afterward. Every insecure request that never
gets made is one that can't be intercepted.

**The plaintext hop is deliberate.** TLS terminates at the ALB; ALB→task is plain HTTP
inside the VPC, between two security groups that only talk to each other, on a network with
no internet route. Terminating at the edge keeps certificate management in one place — ACM,
auto-renewing — instead of baked into every image. For payment or health data I'd
re-encrypt the second hop.

### 1.5 · The target group

[`main.tf:98-121`](networking/main.tf#L98-L121) — where Networking hands off to Compute.

- **`target_type = "ip"`** — Fargate uses `awsvpc` networking, so each task has its own ENI
  and IP; there are no instances to register. The default `instance` type produces a target
  group ECS can't attach to, and you find out at deploy time.
- **`deregistration_delay = 30`** — in-flight requests get 30s to finish when a task is
  replaced. Without a drain period, every deploy kills connections mid-request.
- **`load_balancing_cross_zone_enabled`** — with two tasks, the difference between real
  redundancy and an idle spare.

**The health check is my favorite piece.** It hits `/health`, which on the app side
([`server.go:174`](../server/server-init/server.go#L174)) doesn't just return `200` — it
pings PostgreSQL *and* lists an S3 object, returning `503` if either is down. A shallow
"is the process alive?" check is worse than useless: a task with a dead DB connection still
answers `200`, so the ALB keeps routing to it and every request fails. Here, infrastructure
and application agree on what healthy means, and a broken task removes itself.

**One more validation.** AWS caps target-group `name_prefix` at six characters and rejects
longer ones *at apply time*, leaving you mid-apply with a partial deployment. So
[`variables.tf:105-114`](networking/variables.tf#L105-L114) checks it during plan — same
principle as the NAT rule: **move failures earlier**, where they're free.

### 1.6 · TLS certificates

[`main.tf:129-169`](networking/main.tf#L129-L169) — a genuine chicken-and-egg problem, and
the most interesting part of the module.

ACM won't activate a certificate until you prove domain ownership. Proving it means
publishing DNS records ACM generates *for that specific certificate* — which don't exist
until it's requested. Meanwhile the listener can't start with an unvalidated cert. Four
steps resolve it:

1. **Request the cert**, with `create_before_destroy` so renewal never deletes the live
   certificate before its replacement exists.
2. **Look up the hosted zone** as a `data` block, not a resource — the zone predates this
   infrastructure, so Terraform reads it and a `destroy` can't take the domain down.
3. **Publish validation records** with a `for_each` over the cert's
   `domain_validation_options`, so a multi-domain cert works without an edit.
   `allow_overwrite` makes re-runs reclaim leftover records instead of failing.
4. **Wait**, via `aws_acm_certificate_validation` — a resource that creates nothing and
   blocks until ACM reports `ISSUED`.

The crucial line is back in the listener ([`main.tf:90`](networking/main.tf#L90)):

```hcl
certificate_arn = aws_acm_certificate_validation.this.certificate_arn
```

It references the **validation**, not the certificate. Both expose the same ARN, but
pointing at the validation makes Terraform's dependency graph refuse to build the listener
until validation completes. Ordering guaranteed by the graph, not by hoping.

### 1.7 · What I deleted

The first draft carried a copy of the registry's API Gateway example — ~100 lines with an
Azure JWT authorizer, someone else's Lambda ARNs, a `terraform-aws-modules.modules.tf`
domain, and `allow_origins = ["*"]`. I deleted all of it. An API Gateway in front of an ALB
that already routes to ECS is a second front door doing the first one's job, the authorizer
pointed at an IdP this project doesn't use, and the wildcard CORS contradicted a fix made
earlier in this repo to *deny* cross-origin by default. Shipping it would have meant
infrastructure that looked more impressive and did less.

### Trade-offs at a glance

| Decision | Alternative | Why |
| --- | --- | --- |
| Two AZs | One | Smallest setup that survives a datacenter failure |
| Single NAT | One per AZ | ~$32/mo instead of ~$64; a guarded variable flips it |
| TLS ends at the ALB | End-to-end | Certs managed in one place; hop never leaves the VPC |
| Egress locked to VPC | AWS default (all) | Limits blast radius if the ALB is compromised |
| Deep `/health` | "Process is up" | The ALB's idea of healthy matches the app's |
| `target_type = "ip"` | `instance` | Required by Fargate's `awsvpc` networking |
| No variable defaults | Convenient defaults | Environment-shaping choices stay visible |
| Validation blocks | Let AWS reject it | Fails at plan time, free, not mid-apply |

### Rough edges I'm tracking

- **No access-log bucket.** The ALB writes to `${app_name}-alb-logs`
  ([`main.tf:71-73`](networking/main.tf#L71-L73)) but nothing creates it or grants the ELB
  service write access — so this module can't apply cleanly yet.
- **`app_port` is a variable; the server hardcodes `:8080`**
  ([`server.go:200`](../server/server-init/server.go#L200)). Any other value health-checks a
  dead port. Either the server reads it from the environment or the variable stops
  pretending to be configurable.
- **`outputs.tf` is empty** — and it's what blocks section 2.
- **Tags are inconsistent.** The VPC merges caller tags
  ([`main.tf:25-31`](networking/main.tf#L25-L31)); the ALB hardcodes
  `Environment = "Development"`. Breaks cost allocation.

---

## 2 · Compute

**Planned.** ECS Fargate cluster, ECR registry, task definition, service, IAM roles.

**Fargate over EC2** — ECS on EC2 means patching and scaling instances that exist only to
host containers. For a few small stateless tasks, that's work with no payoff.
**Two tasks, one per AZ**, for the same reason as two AZs, and so ECS can replace one at a
time behind the ALB. **Private subnets, no public IP** — the task security group accepts
traffic only from the *ALB's security group*, referenced by group ID rather than CIDR so it
stays correct if subnet ranges change.

None of that works without the app being stateless, which it already is: metadata in
Postgres, file bytes never touching the server, sessions cached client-side. That's what
makes a task safe to kill and replace at any moment.

**Needs from Networking:** `vpc_id`, `private_subnets`, the target group ARN, and the ALB
security group ID — the four reasons [`outputs.tf`](networking/outputs.tf) has to be filled
in first.

**Still to decide:** ECS `stopTimeout` must sit between the server's 10s graceful shutdown
([`server.go:214-219`](../server/server-init/server.go#L214-L219)) and the target group's
30s drain. Secrets (`JWT_SECRET`, the DSN) must come from Secrets Manager, never plaintext
env vars — those are visible to anyone with `ecs:DescribeTaskDefinition`. Autoscaling once
there's traffic to size against.

---

## 3 · Database

**Planned.** RDS PostgreSQL, DB subnet group, security group.

**Managed over self-hosted** — backups, patching, and failover are solved problems worth
buying, especially where the database is the only stateful component. **Private subnets, no
public accessibility.** **The security group accepts traffic only from the ECS task
security group** on 5432 — not a CIDR, not the VPC range — so the only thing in the account
that can open a connection is an API task. **SSL required**, because the cost is a
connection parameter and the alternative is credentials crossing the network in the clear.

**Open:** Multi-AZ roughly doubles the cost for automatic failover — the same shape of
trade-off as the NAT decision in 1.2, and it deserves the same explicit treatment rather
than a default. Sizing needs real traffic.

---

## 4 · Monitoring

**Planned.** CloudWatch alarms and log groups, wired to SNS.

**Alarm on symptoms first.** The ALB's 5XX rate measures what users actually experience;
CPU matters, but a service can fail every request at 10% CPU. **Log retention must be
finite** — container logs default to never expiring, which is a bill that grows forever for
data nobody reads past week one. And the health check from 1.5 is already load-bearing, so
monitoring's job is to notice the self-healing *happened* — otherwise it's
indistinguishable from quiet degradation.

**Open:** thresholds need a baseline first (alarms set before you know normal either never
fire or fire constantly, and the second trains you to ignore them), and Gin's default log
format should become JSON to make Insights queries useful.

---

## 5 · Cross-cutting

- **Remote state — not configured.** State is on local disk. It needs S3 with DynamoDB
  locking before a second person or a CI job can safely apply. This is the highest-priority
  gap here, because every layer added inherits the risk.
- **Tagging — partly done.** The VPC merges baseline and caller tags correctly; the ALB
  doesn't. See the Networking rough edges.
- **Secrets — app side only.** The server reads `JWT_SECRET`, the DSN, and AWS credentials
  from the environment and fails loudly when they're missing. Sourcing them from Secrets
  Manager arrives with Compute.

---

The through-line: a compromise should have somewhere to stop. Containers sit in subnets
with no route to the internet, behind a load balancer that can talk to nothing but them, in
front of a database that accepts connections from nothing but those containers.
Certificates issue and renew through a dependency chain that can't run out of order. The
health check asks the same question the application does. And the two expensive ways to
misconfigure the network fail at plan time with messages that name the fix.
