# Nimbus CLI

> A cloud file storage system controlled entirely from the terminal — built with production-grade security, a REST API backend, and S3 storage.

[![Development Status](https://img.shields.io/badge/status-under%20development-yellow)](#project-status)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org)

---

## What Is This?

Nimbus is a full-stack cloud storage system I built from scratch. It has two parts: a command-line tool (the CLI) that users interact with, and a server that handles storage, authentication, and data. Files are stored on AWS S3, metadata lives in PostgreSQL, and login sessions are managed through Redis.

The goal was to build something that mirrors how real SaaS products work under the hood — not just a tutorial project, but something with real security, real infrastructure decisions, and end-to-end ownership.

**Key engineering areas this covers:**

| Area | What I built |
| --- | --- |
| API design | REST endpoints with JWT authentication and per-request ownership checks |
| Distributed storage | Files in S3, metadata in PostgreSQL, kept in sync |
| Security | Bcrypt hashing, timing attack mitigation, rate limiting, non-sequential IDs |
| Session management | Redis cache on the client keeps the API fully stateless |
| Infrastructure | Nginx reverse proxy, Docker for local S3 and database emulation |
| CLI experience | Filesystem-style commands that feel native to the terminal |

---

## How It Works

```text
  CLI (your terminal)
        |
        | HTTP
        v
  Nginx (rate limiting + routing)
        |
        v
  API Server (Go/Gin)
      |         |
      v         v
 PostgreSQL    AWS S3
 (metadata)  (files)
```

The CLI never talks to S3 directly. It sends requests to the API, which validates your identity, checks you own the target box, then handles the storage operation. Redis on the client side remembers your session so you stay logged in between commands.

---

## Security Highlights

Built to production standards, not just to pass a code review:

- **Passwords** are hashed with bcrypt (cost 14) and require uppercase, lowercase, number, and special character
- **JWT tokens** expire after 24 hours — every request is verified before anything happens
- **Ownership checks** on every operation — you can only touch your own boxes and files
- **Timing attack mitigation** — login always takes the same time whether the account exists or not, so attackers can't probe for valid emails
- **Non-sequential IDs** — user and box IDs are randomly generated, not `1, 2, 3...`, which prevents enumeration
- **Rate limiting** — Nginx caps auth at 5 req/s and file operations at 10 req/s per IP
- **Audit logging** — failed logins are logged with IP; file operations log user, size, and duration

---

## Data Model

Everything is organized in a three-tier hierarchy:

```text
User
└── Box: "my-project"
    ├── Folder: "documents"
    │   └── resume.pdf
    └── Folder: "code"
        ├── Folder: "nimbus"
        │   └── main.go
        └── notes.txt
```

When you register, a default "Home-Box" is created automatically. You can create more boxes, organize files into folders, and navigate the hierarchy just like a local filesystem.

---

## CLI Commands

| Command | What it does |
| --- | --- |
| `nim login` | Sign in (prompts for email and password) |
| `nim logout` | Sign out and clear local session |
| `nim mkbox <name>` | Create a new box |
| `nim rmbox <name>` | Delete a box and all its contents |
| `nim cb <name>` | Switch to a box |
| `nim cdir <name>` | Create a folder in the current location |
| `nim ls [path]` | List files and folders in the current directory |
| `nim cd <path>` | Navigate into a folder (supports `..` and `/absolute/paths`) |
| `nim pwd` | Show your current location |
| `nim post -f <file>` | Upload a file |
| `nim get -f <file>` | Download a file |
| `nim del -f <file>` | Delete a file |

### Example Session

```bash
nim login
nim mkbox my-project
nim cb my-project
nim cdir documents
nim cd documents
nim post -f resume.pdf
nim ls
# [dir]  documents/
# [file] resume.pdf    145 KB
nim pwd
# my-project/documents
nim logout
```

---

## Quick Start

**Prerequisites:** Go 1.25+, Docker, Redis

```bash
# 1. Clone and start local services (PostgreSQL + S3 emulator)
git clone <repo-url> && cd nim-cli
docker compose up -d

# 2. Configure the server — create server/.env
# PORT=8080
# DB_DSN=postgresql://nimbus:nimbus@localhost:5432/nimbus
# AWS_REGION=us-east-1
# S3_BUCKET=nimbus-storage
# S3_ENDPOINT=http://localhost:4566
# S3_FORCE_PATH_STYLE=true
# JWT_SECRET=your-secret-key

# 3. Start the API server
cd server && go run main.go

# 4. Build and run the CLI
cd client && go build -o nim cli/main.go
./nim --help
```

---

## Tech Stack

| Component | Technology |
| --- | --- |
| CLI | Go + Cobra |
| API Server | Go + Gin |
| Database | PostgreSQL + GORM |
| File Storage | AWS S3 / LocalStack |
| Session Cache | Redis |
| Reverse Proxy | Nginx |
| Local Dev | Docker Compose |

---

## Project Status

**Nimbus is under active development.**

Done:

- User registration and login with JWT
- File upload, download, and delete
- Folder creation and listing with nested path support
- Box creation and deletion
- Full path navigation (`cd`, `pwd`, `ls`)
- Nginx reverse proxy with rate limiting

In progress:

- Folder delete, move, and rename
- Box listing command

Planned:

- File move and rename
- Pre-signed S3 URLs
- File versioning
- Sharing and collaboration

---

## License

MIT
