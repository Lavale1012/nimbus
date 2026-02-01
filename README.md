# Nimbus CLI

> A production-style, cloud-native file storage system built to demonstrate real-world backend engineering: authentication, distributed storage, session management, and secure multi-tenant access — all controlled from a CLI.

[![Development Status](https://img.shields.io/badge/status-under%20development-yellow)](#project-status)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)

---

## What This Project Demonstrates

Nimbus is a full-stack cloud storage system — a Go CLI client paired with a REST API server — that manages files on S3 through a terminal-native workflow. It was built to mirror real SaaS backend architecture and demonstrate end-to-end ownership of both client tooling and server infrastructure.

**Core engineering competencies:**

- **API design & authentication** — JWT-based auth with per-request ownership validation and secure credential handling
- **Distributed storage** — metadata in PostgreSQL, binary data in S3, kept in sync through transactional handlers
- **Session management** — client-side Redis cache keeps the API stateless while giving the CLI a persistent working context
- **Security** — bcrypt (cost 14), constant-time comparisons, non-sequential IDs, Nginx rate limiting, timing attack mitigation
- **Infrastructure** — Nginx reverse proxy, Dockerized local environment with LocalStack for S3 emulation
- **CLI UX** — filesystem-like commands (`cd`, `pwd`, `post`, `get`) that abstract cloud operations into familiar patterns

---

## Architecture

Nimbus follows a decoupled architecture where each component has a single responsibility. The CLI stays lightweight and delegates all business logic to the API. PostgreSQL owns structured metadata and relationships. S3 handles large binary data. Redis caches session state on the client side so the API remains stateless.

```text
+------------------+       +------------------+
|    CLI Client    |<----->|      Redis       |
|    (Go/Cobra)    |       | (session cache)  |
+--------+---------+       +------------------+
         |
    HTTP | requests
         v
+------------------+
|      Nginx       |
|  reverse proxy   |
|  (rate limiting) |
+--------+---------+
         |
         v
+------------------+       +------------------+       +------------------+
|                  |  SQL  |                  |  S3   |                  |
|   PostgreSQL     |<----->|   API Server     |<----->|     AWS S3       |
|   (metadata)     |       |   (Go/Gin)       |       |   (file storage) |
|                  |       |                  |       |                  |
+------------------+       +------------------+       +------------------+
```

| Layer          | Technology          | What it does                                            |
| -------------- | ------------------- | ------------------------------------------------------- |
| CLI            | Go + Cobra          | User-facing commands (`nim post`, `nim cd`, etc.)       |
| Reverse Proxy  | Nginx               | Rate limiting (5-10 req/s), timeouts, request routing   |
| API Server     | Go + Gin            | REST endpoints, JWT auth, request validation            |
| Database       | PostgreSQL + GORM   | Users, boxes, folders, file metadata                    |
| File Storage   | AWS S3 / LocalStack | Durable object storage for uploaded files               |
| Session Cache  | Redis               | Stores JWT token, current box, and working path locally |
| Infrastructure | Docker Compose      | Runs PostgreSQL and LocalStack for local development    |

---

## Security

Security decisions were made to reflect production standards, not just check boxes:

- **Password hashing**: Bcrypt at cost 14. Enforces minimum 8 characters with uppercase, lowercase, number, and special character requirements.
- **Authentication**: JWT tokens with 24-hour expiration. Every file and folder operation requires a valid token.
- **Ownership validation**: All operations verify the authenticated user owns the target box before proceeding. No implicit trust between resources.
- **Timing attack mitigation**: Login compares against a dummy hash for nonexistent users so response time doesn't leak whether an account exists.
- **Non-sequential IDs**: User IDs are random 8-digit numbers. Box IDs use 63-bit cryptographically secure random generation. Sequential IDs would expose user count and allow enumeration.
- **Rate limiting**: Nginx limits auth endpoints to 5 req/s and file endpoints to 10 req/s per IP.
- **Audit logging**: Failed login attempts are logged with IP address. File operations log user ID and duration.

---

## Data Model

Files are organized in a three-tier hierarchy — Boxes contain Folders, Folders contain Files — with full nesting support:

```text
User (ID: 45892034)
+-- Box: "Home-Box"
    +-- Folder: "projects"
    |   +-- Folder: "nimbus-cli"
    |   |   +-- main.go
    |   |   +-- README.md
    |   +-- Folder: "website"
    |       +-- index.html
    +-- Folder: "documents"
        +-- resume.pdf
```

On registration, the system generates a random 8-digit user ID and creates a default "Home-Box". From there, users create folders, navigate paths, and upload or download files through the CLI.

---

## CLI Commands

| Command      | Usage                                    | Description                                                                |
| ------------ | ---------------------------------------- | -------------------------------------------------------------------------- |
| `nim login`  | `nim login`                              | Authenticate with your Nimbus account (interactive email/password prompt)  |
| `nim logout` | `nim logout`                             | Clear your local session                                                   |
| `nim cb`     | `nim cb Home-Box`                        | Set the active box for subsequent operations                               |
| `nim post`   | `nim post -f ./file.txt -d path/to/dest` | Upload a file to the current box (optional destination path)               |
| `nim get`    | `nim get -f <file> -o ./output.txt`      | Download a file (box and path resolved from session cache)                 |
| `nim del`    | `nim del -f <file>`                      | Delete a file                                                              |
| `nim cdir`   | `nim cdir my-folder [parent/path]`       | Create a new folder in the current box                                     |
| `nim cd`     | `nim cd path/to/folder`                  | Change working directory within the box (supports `..` and absolute paths) |
| `nim pwd`    | `nim pwd`                                | Print the current box and working directory                                |

### Example Workflow

```bash
# Log in
nim login

# Set your active box
nim cb Home-Box

# Create a folder structure
nim cdir projects
nim cd projects
nim cdir reports

# Upload a file into the current path
nim post -f quarterly-report.pdf -d reports

# Check where you are
nim pwd
# Output: Home-Box/projects

# Download a file (box and path resolved from session cache)
nim get -f quarterly-report.pdf -o ./local-copy.pdf

# Delete a file
nim del -f quarterly-report.pdf

# Log out
nim logout
```

---

## API Endpoints

Base URL: `http://localhost:8080/v1/api`

### Authentication

| Method | Endpoint               | Description                                            |
| ------ | ---------------------- | ------------------------------------------------------ |
| POST   | `/auth/users/register` | Register a new user (email, password, 4-digit passkey) |
| POST   | `/auth/users/login`    | Log in and receive a JWT token                         |

### Files (requires Bearer token)

| Method | Endpoint                                 | Description     |
| ------ | ---------------------------------------- | --------------- |
| POST   | `/files?box_name={name}&filePath={path}` | Upload a file   |
| GET    | `/files?box_name={name}&key={s3_key}`    | Download a file |
| DELETE | `/files/{s3_key}`                        | Delete a file   |

### Folders (requires Bearer token)

| Method | Endpoint                                                  | Description     |
| ------ | --------------------------------------------------------- | --------------- |
| POST   | `/folders?box_name={name}&path={path}&folder_name={name}` | Create a folder |

---

## Quick Start

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- Redis (running locally on port 6379)

### 1. Clone and start services

```bash
git clone <repo-url>
cd nim-cli

# Start PostgreSQL and LocalStack (S3 emulator)
docker compose up -d
```

### 2. Configure environment

Create a `.env` file in the server directory (or use the existing one):

```env
PORT=8080
LOCAL_DEV=true
DATABASE_URL=postgresql://nimbus:nimbus@localhost:5432/nimbus
AWS_REGION=us-east-1
S3_BUCKET=nimbus-storage
S3_ENDPOINT=http://localhost:4566
S3_FORCE_PATH_STYLE=true
JWT_SECRET=your-secret-key
```

### 3. Start the API server

```bash
cd server
go run main.go
```

### 4. Build and use the CLI

```bash
cd client
go build -o nim cli/main.go

# Optionally add to your PATH
sudo mv nim /usr/local/bin/

# Verify it works
nim --help
```

---

## Development

### Building

```bash
# Build CLI
cd client && go build -o nim cli/main.go

# Build API server
cd server && go build -o api-server main.go
```

### Testing

```bash
# Run all server tests
cd server && go test ./...

# Run with coverage
cd server && go test -cover ./...

# Run specific test file
cd server && go test -v ./tests/
```

### Code quality

```bash
go fmt ./...
go vet ./...
```

---

## Project Structure

```text
nim-cli/
|-- client/
|   |-- cli/
|   |   |-- main.go              # CLI entry point
|   |   |-- cmd/                  # Cobra command definitions
|   |   |   |-- root.go
|   |   |   |-- login.go
|   |   |   |-- logout.go
|   |   |   |-- post.go          # File upload
|   |   |   |-- get.go           # File download
|   |   |   |-- delete.go        # File deletion
|   |   |   |-- box.go           # Set current box
|   |   |   |-- folder.go        # Create folder
|   |   |   +-- path.go          # cd, pwd, ls commands
|   |   |-- animations/          # Loading spinners and progress bars
|   |   |-- types/               # Shared type definitions
|   |   +-- banner/              # Login banner display
|   |-- cache/
|   |   +-- redis.go             # Redis session management
|   +-- utils/
|       +-- helpers/             # Login status checks
|
|-- server/
|   |-- main.go                  # Server entry point
|   |-- server-init/
|   |   +-- server.go            # Gin setup, route registration, S3/DB init
|   |-- handlers/
|   |   |-- user/auth.go         # Registration and login logic
|   |   |-- file/file.go         # Upload, download, delete handlers
|   |   |-- folder/folder.go     # Folder creation handler
|   |   +-- box/box.go           # Box handlers (stubs)
|   |-- routes/                  # Route group definitions
|   |-- models/                  # GORM models (User, Box, Folder, File)
|   |-- middleware/jwt/          # JWT creation, verification, auth middleware
|   |-- db/
|   |   |-- s3/                  # S3 client connection
|   |   +-- postgres/            # PostgreSQL connection and auto-migration
|   |-- utils/                   # Hashing, ID generation, helper functions
|   |-- tests/                   # Unit and integration tests
|   +-- infra/nginx/             # Nginx reverse proxy config
|
|-- docker-compose.yml           # PostgreSQL + LocalStack
+-- CLAUDE.md
```

---

## Project Status

Nimbus is under active development.

### Implemented

- User registration and login with JWT authentication
- File upload, download, and delete via S3
- Folder creation with nested path support
- Path navigation (`cd`, `pwd`) within boxes
- Redis-based session caching on the client
- Nginx reverse proxy with rate limiting
- CLI commands: `login`, `logout`, `post`, `get`, `del`, `cb`, `cdir`, `cd`, `pwd`

### Partially implemented

- Folder operations (create works; list, move, rename, delete are stubbed)
- Box management (routes defined; handlers not yet built)

### Planned

- `ls` command for listing directory contents
- File move and rename
- Box creation and deletion via CLI
- Pre-signed S3 URLs for direct uploads
- File versioning and duplicate detection
- Collaboration and sharing features
- File encryption

---

## License

MIT
