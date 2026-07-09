<h1 align="center">☁️ Nimbus CLI</h1>

<p align="center">
  <strong>A cloud file-storage platform you drive entirely from your terminal.</strong><br>
  Go CLI + REST API · direct-to-S3 transfers · PostgreSQL metadata · Redis sessions ·
  containerized and deployed on AWS ECS Fargate via Terraform.
</p>

<p align="center">
  <a href="#project-status"><img src="https://img.shields.io/badge/status-active%20development-yellow" alt="status"></a>
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go 1.26+">
  <a href=".github/workflows/main.yml"><img src="https://github.com/Lavale1012/nimbus/actions/workflows/main.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/AWS-ECS%20Fargate%20%7C%20RDS%20%7C%20S3-FF9900?logo=amazon-aws&logoColor=white" alt="AWS">
  <img src="https://img.shields.io/badge/IaC-Terraform-7B42BC?logo=terraform&logoColor=white" alt="Terraform">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT">
</p>

<p align="center">
  <a href="#-at-a-glance">At a Glance</a> ·
  <a href="#-architecture">Architecture</a> ·
  <a href="#-aws-infrastructure">Infrastructure</a> ·
  <a href="#-security">Security</a> ·
  <a href="#-using-the-cli">Using the CLI</a> ·
  <a href="#-quick-start">Quick Start</a> ·
  <a href="#-tech-stack">Tech Stack</a>
</p>

---

## ⚡ At a Glance

Most side projects stop at "it works on my machine." Nimbus answers a harder
question: **what does it take to ship a secure, multi-user cloud service end to
end?** Every layer — CLI, stateless API, auth model, AWS infrastructure, and the
CI/CD that guards it — was designed and built from scratch.

| Area | What's demonstrated |
| --- | --- |
| **Distributed systems** | Stateless, horizontally scalable API; file bytes bypass the server via presigned S3 URLs; PostgreSQL metadata kept in sync with object storage |
| **Security engineering** | bcrypt (cost 14) · JWT with `alg:none` rejection · Redis-backed per-IP + per-email rate limiting · timing-attack-resistant auth · deny-by-default CORS · ownership checks on every request |
| **Cloud infrastructure** | Full AWS stack as Terraform IaC ([separate repo](https://github.com/Lavale1012/aws-cloud-suite)) — VPC across 2 AZs, ECS Fargate (HA), ALB, RDS PostgreSQL, ECR, CloudWatch + SNS alarms, S3 remote state with DynamoDB locking |
| **CI/CD & quality** | GitHub Actions gate on every PR: lint, race-tested units, build, dependency-CVE scan, secret scan ([overview](.github/workflows/README.md) · [deep dive](.github/CICD.md)) |
| **Developer experience** | Filesystem-style commands (`cd`, `ls`, `pwd`), live progress bars, one-command local stack via Docker Compose |

> 📖 Want to see every command in action? Follow the [full feature demo](DEMO.md).

---

## 🏗 Architecture

```text
   Your terminal (nim CLI)
            │  HTTPS
            ▼
   ┌──────────────────────┐        presigned URL (returned to CLI)
   │  ALB  (AWS)           │ ───────────────────────────────────────┐
   └──────────┬───────────┘                                         │
              │ :8080                                               │
              ▼                                                     ▼
   ┌──────────────────────┐        metadata          ┌──────────────────────┐
   │  API  (Go / Gin)     │ ───────────────────────► │  RDS PostgreSQL       │
   │  on ECS Fargate ×2   │                          └──────────────────────┘
   └──────────┬───────────┘
              │ auth + ownership only
              ▼
   ┌──────────────────────┐        direct upload/download (CLI ⇄ S3)
   │  AWS S3              │ ◄──────────────────────────────────────────────┐
   └──────────────────────┘                                                │
                                                        (bytes never touch the API)
```

**The key design decision: file data never flows through the API server.** The
server authenticates the request, verifies you own the target box, and hands
back a short-lived presigned S3 URL. The CLI streams the file directly to or
from S3 — the API only ever moves small JSON, never gigabytes of user data.
That keeps it fast, cheap, and horizontally scalable.

Redis on the client side caches your session between commands, which is what
lets the API stay fully stateless.

### Data model

```text
User
└── Box: "my-project"          ← top-level container (a "Home-Box" is created at signup)
    ├── Folder: "documents"
    │   └── resume.pdf
    └── Folder: "code"
        └── notes.txt
```

You navigate boxes and folders exactly like a local filesystem — `cd`, `ls`,
`pwd`, relative and absolute paths.

---

## ☁️ AWS Infrastructure

Production is defined entirely as Terraform IaC — four modules (`networking`,
`compute`, `database`, `monitoring`), with state in an **S3 backend with
DynamoDB locking** so changes are safe to apply as a team.

> 🏗 The Terraform code lives in its own repo:
> **[Lavale1012/aws-cloud-suite](https://github.com/Lavale1012/aws-cloud-suite)**

<p align="center">
  <img src="readmeImages/aws-architecture.jpeg" alt="Nimbus AWS architecture — nim CLI to ALB to ECS Fargate across two Availability Zones, with RDS PostgreSQL, S3 direct transfers, ECR, NAT Gateway, and CloudWatch/SNS monitoring" width="900">
</p>

| Layer | Resources |
| --- | --- |
| **Networking** | VPC (`10.0.0.0/16`) · 2 public + 2 private subnets across 2 AZs · Internet Gateway · NAT Gateway + Elastic IP |
| **Compute** | ECS Fargate cluster — **2 API tasks** for HA in private subnets (no public IPs) · ALB · ECR · IAM task-execution role · tiered security groups |
| **Database** | RDS PostgreSQL 15 · private DB subnet group · security group locked to **ECS traffic only** · SSL required |
| **Monitoring** | CloudWatch alarms (ECS CPU, RDS CPU, ALB 5XX) → SNS email · 7-day log retention |

**Network security posture:** the only inbound path is
`Internet → ALB → ECS on :8080`. API tasks have no public IPs, the database
only accepts connections from the ECS security group, and outbound traffic
(e.g. image pulls) routes through the NAT Gateway.

> **Deploy flow:** build the API image → push to ECR → ECS Fargate rolls out
> the new task definition behind the ALB, which health-checks `/health` before
> routing traffic.

---

## 🔐 Security

Built to production standards, not just to pass a code review:

- **Passwords** — bcrypt (cost 14); uppercase, lowercase, number, and special character required
- **Passkey-based password reset** — a per-user bcrypt-hashed passkey (set at registration) authorizes self-service reset, no email/SMS channel needed
- **JWT tokens** — 24-hour expiry, verified on every request, non-HMAC (`alg:none`) tokens rejected
- **Ownership checks** — every operation verifies you own the target box, folder, or file
- **Timing-attack mitigation** — login and reset take constant time whether the account exists or not, so attackers can't probe for valid emails
- **Rate limiting** — login and password reset throttled per-IP **and** per-email (5 attempts / 15 min), backed by Redis so the limit holds across all API instances
- **Deny-by-default CORS** — outside local dev, cross-origin requests are rejected unless an explicit allowlist is configured
- **Non-sequential IDs** — user and box IDs are randomly generated, preventing enumeration
- **Presigned S3 URLs** — file transfers use time-limited, scoped credentials (15-min expiry)
- **Audit logging** — failed logins and resets logged with IP; file operations log user, size, and duration

---

## 💻 Using the CLI

```bash
nim login
nim mkbox my-project
nim cb my-project
nim cdir documents
nim post -f resume.pdf -d documents/resume.pdf
nim ls
# [dir]  documents/
# [file] resume.pdf    145 KB
nim logout
```

<details>
<summary><strong>Full command reference</strong> (click to expand)</summary>

| Command | What it does |
| --- | --- |
| `nim register` | Open the registration page to create an account (email, password, passkey) |
| `nim login` | Sign in (type `r` at the email prompt to reset your password via passkey) |
| `nim logout` | Sign out and clear local session |
| `nim mkbox <name>` | Create a new box |
| `nim rmbox <name>` | Delete a box and all its contents |
| `nim bls` | List all your boxes |
| `nim cb <name>` | Switch to a box |
| `nim cdir <name> [destination]` | Create a folder in the current box |
| `nim ls [path]` | List files and folders |
| `nim cd <path>` | Navigate into a folder (supports `..` and `/absolute/paths`) |
| `nim pwd` | Show your current location |
| `nim post -f <file> [-d <dest>]` | Upload a file (direct to S3 via presigned URL) |
| `nim get -f <key> [-o <output>]` | Download a file (direct from S3 via presigned URL) |
| `nim del -f <key>` | Delete a file |
| `nim rename --key <key> --name <new>` | Rename a file |
| `nim mv --key <key> --to <folder>` | Move a file to a different folder |
| `nim rmdir <name>` | Delete a folder and all its contents |
| `nim mvdir <name> <new-name>` | Rename a folder |

</details>

See [DEMO.md](DEMO.md) for a guided walkthrough of every command.

---

## 🚀 Quick Start

**Prerequisites:** Go 1.26+, Docker, Redis

```bash
# 1. Clone and start local services (PostgreSQL + S3 emulator)
git clone https://github.com/Lavale1012/nimbus.git && cd nimbus
docker compose up -d

# 2. Configure the server — create a .env file (see server/utils/getEnv.go for lookup order)
# LOCAL_DEV=true
# DATABASE_URL=host=localhost user=nimbus password=nimbus dbname=nimbus port=5432 sslmode=disable
# AWS_REGION=us-east-1
# S3_BUCKET=nimbus-storage
# S3_ENDPOINT=http://localhost:4566          # read by the AWS SDK for LocalStack
# S3_FORCE_PATH_STYLE=true                    # read by the AWS SDK for LocalStack
# JWT_SECRET=your-secret-key                  # required; use a long random value
# CORS_ORIGINS=http://localhost:3000          # optional; denied by default outside LOCAL_DEV

# 3. Start the API server (listens on :8080)
cd server && go run main.go

# 4. Build and run the CLI
cd client && go build -o nim cli/main.go
./nim --help
```

---

## 🧰 Tech Stack

| Layer | Technology |
| --- | --- |
| CLI | Go · Cobra · progressbar (live progress UI) |
| API Server | Go · Gin |
| Database | PostgreSQL · GORM (RDS in production) |
| File Storage | AWS S3 (presigned URLs) · LocalStack for local dev |
| Sessions & Rate Limiting | Redis |
| Compute | AWS ECS Fargate (2 tasks, HA) behind an ALB |
| Infrastructure | Terraform (VPC, Fargate, RDS, ECR, NAT, CloudWatch, SNS) · S3 + DynamoDB remote state |
| CI/CD | GitHub Actions — lint, race-tested tests, build, govulncheck, gitleaks |
| Local Dev | Docker Compose (PostgreSQL + LocalStack S3) |

---

## 📌 Project Status

**Under active development.**

**Done** ✅

- User registration, JWT login, and passkey-based password reset
- Redis-backed per-IP + per-email rate limiting on auth endpoints
- File upload, download, delete, rename, and move — all via presigned S3 URLs
- Folder and box management, full path navigation (`cd`, `pwd`, `ls`), zip download
- Live progress bars and spinners on all CLI commands
- Comprehensive server-side tests (handlers, auth, file ops, box ops)
- ALB-ready server — trusted proxy headers, deny-by-default CORS, HTTP timeouts, body limits
- Production AWS infrastructure as Terraform IaC
- CI/CD pipeline gating every PR ([details](.github/workflows/README.md))

**Planned** 🔜

- File versioning
- Sharing and collaboration
- Cross-platform build scripts and releases

---

## License

MIT
