# Nimbus Infrastructure — A Guided Tour

The AWS infrastructure behind Nimbus, written as code with Terraform. One section per
layer, explaining what it builds, why I built it that way, and what the choice buys.

| § | Layer | Status |
| --- | --- | --- |
| [1](#1--networking) | **Networking** — VPC, subnets, NAT, ALB, TLS | **Built** |
| [2](#2--ecr) | **ECR** — image registry, lifecycle policy | **Built** |
| [3](#3--compute) | **Compute** — ECS Fargate cluster, service, task definition | **Built** |
| [4](#4--database) | **Database** — RDS PostgreSQL | Planned |
| [5](#5--monitoring) | **Monitoring** — CloudWatch, SNS | Planned |
| [6](#6--cross-cutting) | **Cross-cutting** — state, tagging, secrets | Partly built |

Networking, ECR, and Compute are built — the path from an image in a registry to a task
serving traffic behind TLS is complete. Database and Monitoring are documented with the
decisions already made and what they attach to. I'd rather ship a deliberate foundation
than a broad `apply` that works by accident.

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
    ECS Fargate tasks — no public IP        ◄── image pulled from ECR
       │  :5432, security-group-locked          via the NAT gateway
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

### Rough edges — since closed

The four gaps this section used to list are all fixed, and how they were fixed is the more
useful documentation:

- **The access-log bucket now exists.** [`main.tf:44`](networking/main.tf#L44) provisions it
  with the ELB service policy attached, and the ALB `depends_on` it
  ([`main.tf:89`](networking/main.tf#L89)) so the listener can't come up before the bucket
  it logs to. `alb_logs_force_destroy` defaults to false: a destroy fails loudly on a bucket
  that still holds logs rather than silently emptying it.
- **`app_port` is no longer a lie.** The server reads `PORT`
  ([`server.go:203-213`](../server/server-init/server.go#L203-L213)) and compute injects it
  from the same variable the target group health-checks. A malformed value is rejected at
  startup rather than falling back, because binding the wrong port surfaces as an
  unexplained health-check loop instead of an error.
- **`outputs.tf` is filled** — ten outputs, and they're what made sections 2 and 3
  possible. Subnet outputs are named `_ids` while the input variables of the same shape
  carry CIDRs, so `private_subnet_ids = module.networking.private_subnets` can't read like
  a mistake at the call site.
- **Tags are consistent.** Every resource in the module now carries `local.tags`, and ECR
  and Compute use the identical shape so a cost report groups all three layers by the same
  keys.

---

## 2 · ECR

**Built.** [`ecr/`](ecr/) — the registry the task pulls its image from.

**Why it's a separate module from Compute.** A repository holds build artifacts that
outlive any particular deployment. `terraform destroy` on the service shouldn't take the
images with it, and the repository has to exist *and already contain an image* before an
ECS service can start a task — fold them into one apply and the service's first create
races an empty repository and fails the pull. Splitting them makes that ordering explicit
instead of a race you rediscover at 2am.

**`IMMUTABLE` tags.** A tag, once pushed, permanently refers to the same bytes. That's what
makes a rollback trustworthy: redeploying a known-good tag gets the image that tag was
*tested against*, not whatever was pushed over it since. The cost is real — CI can't push a
moving `:latest`, so images are tagged by commit SHA or version.

**A lifecycle policy, because storage bills forever.** Two rules
([`main.tf:56-83`](ecr/main.tf#L56-L83)): expire untagged images after 7 days (they're
layers orphaned by a re-push, referenced by nothing, billed per GB-month like everything
else), then keep only the 30 most recent. Rules evaluate in priority order and an image
matched by one is not considered by any later rule, which is why the `tagStatus = "any"`
catch-all has to sit last.

The policy is *required* rather than optional here: the upstream module sets
`create_lifecycle_policy = true` by default but leaves the document empty, and AWS rejects
an empty policy at apply time.

**One sizing trap worth knowing:** `max_image_count` must stay comfortably above your
rollback window. The rule deletes by age, and a Fargate task re-pulls its image on restart
— so expiring the image a *running* service uses breaks that task's next start, not just
the rollback.

---

## 3 · Compute

**Built.** [`compute/`](compute/) — ECS Fargate cluster, service, task definition, and the
security group rules that connect it to everything else.

**Fargate over EC2** — ECS on EC2 means patching and scaling instances that exist only to
host containers. For a few small stateless tasks, that's work with no payoff. **Private
subnets, `assign_public_ip = false`** — the task is reachable only through the load
balancer, while outbound still works via the NAT gateway, which is how the image gets
pulled from ECR and how the app reaches S3.

None of that works without the app being stateless, which it already is: metadata in
Postgres, file bytes never touching the server, sessions cached client-side. That's what
makes a task safe to kill and replace at any moment.

### 3.1 · The rules the upstream module doesn't write for you

The ECS module creates a task security group but adds **no rules** — both rule maps default
to `{}`. Left empty, the ALB can't reach the task, health checks never pass, and the task
can't pull its own image. So ingress is one rule
([`main.tf:77-88`](compute/main.tf#L77-L88)) allowing the app port from
`referenced_security_group_id` — the ALB's security group **by ID, not CIDR**, so it stays
correct if subnet ranges change and nothing else in the VPC can open a connection.

Egress stays wide (`0.0.0.0/0`) by necessity: ECR, CloudWatch Logs, and S3 are public
endpoints reached through the NAT gateway. VPC endpoints would let this narrow to the VPC
CIDR the way the ALB's egress does — worth doing once traffic justifies the per-endpoint
hourly cost. Documented as a deliberate gap rather than left to look accidental.

### 3.2 · Spot capacity without a zero-task failure mode

```hcl
default_capacity_provider_strategy = {
  FARGATE      = { base = var.on_demand_base, weight = var.on_demand_weight }
  FARGATE_SPOT = { weight = var.spot_weight }
}
```

`base` is the number of tasks pinned to a provider **before** weight applies; weight only
splits what's left. `base = 1` keeps one task on capacity AWS can't reclaim, so a Spot
interruption can never take the service to zero. Spot is roughly 70% cheaper but AWS can
reclaim a task with two minutes' notice — this buys most of the discount without betting
availability on it.

The registry example uses `base = 20`, which sends the first twenty tasks to on-demand. At
a two-task scale that means `FARGATE_SPOT` would never run at all — a copied default that
silently does nothing.

### 3.3 · Three numbers that have to agree

The most satisfying part of this layer is a chain that was previously three
near-misses:

| Value | Where | Why |
| --- | --- | --- |
| 10s | server's graceful shutdown ([`server.go`](../server/server-init/server.go)) | drains in-flight requests |
| 20s | `stop_timeout` ([`main.tf:137`](compute/main.tf#L137)) | SIGTERM → SIGKILL window |
| 30s | target group `deregistration_delay` (networking) | ALB stops sending new requests |

`stop_timeout` must sit **between** the other two. The upstream module defaults it to 120,
which strands a container for 90 seconds after both the app and the load balancer are
finished with it — lengthening every single deploy for nothing.

The same closing-the-loop applies to the port: the target group health-checks `app_port`,
the port mapping advertises it, and the container definition injects it as `PORT`
([`main.tf:111-114`](compute/main.tf#L111-L114)) so the server actually binds it. Before
this, `app_port` was three places agreeing on a number the process ignored.

### 3.4 · Costs pinned rather than inherited

**Container Insights is set explicitly** to `disabled` — the upstream module turns it on by
default and it bills per metric collected. It should be a decision, not something inherited.
**Log retention is 14 days**, matched deliberately to networking's `alb_log_retention_days`
so an incident spanning both layers has request logs and application logs covering the same
window. **Secrets resolve by ARN** through `task_exec_secret_arns`, never as plaintext
environment variables — those are readable by anyone holding
`ecs:DescribeTaskDefinition`.

**Still open here:** `readonly_root_filesystem` defaults to true (stricter than AWS's own
default), but it's a runtime failure rather than a plan error — a binary that writes to disk
crashes on first write, so it needs confirming against the real container.
[`compute/outputs.tf`](compute/outputs.tf) is still empty; CI will need the cluster and
service names to force a new deployment.

---

## 4 · Database

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

## 5 · Monitoring

**Planned.** CloudWatch alarms and log groups, wired to SNS.

**Alarm on symptoms first.** The ALB's 5XX rate measures what users actually experience;
CPU matters, but a service can fail every request at 10% CPU. And the health check from 1.5
is already load-bearing, so monitoring's job is to notice the self-healing *happened* —
otherwise it's indistinguishable from quiet degradation.

Log retention is already handled where the logs are produced: 14 days on both the ALB
access logs and the container logs, matched so an incident spanning both layers has the
same window of each. What's missing is the alarms.

**Open:** thresholds need a baseline first (alarms set before you know normal either never
fire or fire constantly, and the second trains you to ignore them), and Gin's default log
format should become JSON to make Insights queries useful.

---

## 6 · Cross-cutting

- **Remote state — still not configured.** State is on local disk. It needs S3 with
  DynamoDB locking before a second person or a CI job can safely apply. This is now the
  highest-priority gap in the directory by some distance, because every layer added
  inherits the risk and there are three of them.
- **Tagging — done.** All three modules build `local.tags` the same way: a
  `Terraform`/`Environment` baseline merged under caller-supplied tags. Identical shape
  across layers so a cost report groups them by the same keys.
- **Version pinning — done, with a caveat worth stating.** Every module pins its provider
  (`~> 6.0`) and its Terraform floor (`>= 1.9`, where a validation block may reference
  another variable). But `.terraform.lock.hcl` locks *providers only* — registry module
  versions aren't captured by it, so the `version` argument on each module block is the
  only thing stopping a later `terraform init` from resolving a different major release.
- **Secrets — wired, not yet populated.** Compute accepts `task_exec_secret_arns` so the
  task definition resolves secrets by ARN, and `container_environment` is documented as
  the wrong place for them. Creating the actual Secrets Manager entries for `JWT_SECRET`
  and the database DSN lands with the Database layer.
- **No root module yet.** Each layer is applied on its own and wired by hand through
  outputs. A root composition that passes `module.networking.private_subnet_ids` into
  compute is the natural next step, and it's what makes the dependency order enforceable
  rather than remembered.

---

The through-line: a compromise should have somewhere to stop. Containers sit in subnets
with no route to the internet, behind a load balancer that can talk to nothing but them, in
front of a database that accepts connections from nothing but those containers.
Certificates issue and renew through a dependency chain that can't run out of order. The
health check asks the same question the application does. And the expensive ways to
misconfigure this fail at plan time with messages that name the fix.

The second through-line, visible now that three layers exist: **the upstream defaults are
usually the bug.** An empty ECR lifecycle policy that AWS rejects, an ECS security group
with no rules, `base = 20` on a two-task service, a 120-second stop timeout, Container
Insights billing quietly by default, logs that never expire. Every one of those applies
cleanly and looks fine in a plan. Most of the work in these modules was reading what the
module does when you don't tell it anything, and then telling it something.
