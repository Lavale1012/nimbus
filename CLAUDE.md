# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Nimbus CLI is a cross-platform command-line interface for cloud file storage and management written in Go. The system uses a hierarchical data organization:

- **Sections** → top-level containers (e.g., "work", "school", "personal")  
- **Boxes** → containers within a section (e.g., "fall-2025", "photos")
- **Folders** → hierarchical directories within a box
- **Files** → leaf objects, optionally versioned

## Architecture

**Tech Stack:**
- Go for CLI and Backend API (Cobra for CLI, Gin for API)
- PostgreSQL for metadata storage
- AWS S3 for object storage with pre-signed URLs
- OIDC authentication (Auth0/Cognito/Clerk) with RBAC
- Docker Compose for local development (PostgreSQL + LocalStack)

**Key Design Principles:**
- File bytes flow directly to S3 via pre-signed URLs (API never proxies file content)
- Path semantics: `/SectionName/BoxName:/folder/path` format
- Local development uses stubbed auth with `LOCAL_DEV=true`

## Development Commands

Since this is a Go project, use standard Go tooling:

```bash
# Build the CLI
go build -o nimbus ./cli/cmd/nimbus

# Build the API server  
go build -o api-server ./service/cmd/api

# Run tests
go test ./...

# Run tests for specific package
go test ./cli/internal/api
go test ./service/internal/meta

# Format code
go fmt ./...

# Lint (if golangci-lint is configured)
golangci-lint run

# Start local development environment
docker compose up -d

# Run database migrations (when implemented)
go run ./service/migrations migrate up
```

## Project Structure

```
nimbus/
├── cli/
│   ├── cmd/nimbus/           # Cobra commands (whoami, section, box, ls, upload, download)
│   ├── internal/auth/        # Device flow, token storage (keychain)
│   ├── internal/api/         # REST client + DTOs
│   ├── internal/transfer/    # HTTP PUT/GET for pre-signed URLs, progress, retries
│   └── internal/path/        # Parse & resolve /Section/Box:/folder paths
├── service/
│   ├── cmd/api/              # API server entry point
│   ├── internal/httpserver/  # Gin setup, middleware
│   ├── internal/auth/        # JWT validation (stubbed in local dev)
│   ├── internal/storage/     # S3 pre-signed URL generation
│   ├── internal/meta/        # Database repositories (GORM/sqlc)
│   ├── internal/resolve/     # Path→ID resolution logic
│   ├── pkg/types/            # Shared request/response DTOs
│   └── migrations/           # Database schema migrations
├── infra/
│   ├── docker-compose.yml    # PostgreSQL + LocalStack
│   └── localstack-init/      # S3 bucket initialization
└── .env.example
```

## Key APIs

**Base URL:** `http://localhost:8080/v1`

Core endpoints:
- `GET /healthz` - Health check
- `GET /users/me` - Current user info
- `POST /sections`, `GET /sections` - Section management
- `POST /boxes`, `GET /sections/:id/boxes` - Box management
- `POST /folders`, `GET /boxes/:id/list` - Folder operations
- `POST /files/presign-upload` - Get pre-signed upload URL
- `POST /files/:id/complete` - Finalize upload
- `GET /files/:id/presign-download` - Get pre-signed download URL
- `GET /resolve?path=/Section/Box:/folder/path` - Resolve path to IDs

## CLI Commands

```bash
nimbus whoami                                    # Current user info
nimbus section create "school"                  # Create section
nimbus section ls                                # List sections
nimbus box create "fall-2025" --section "school" # Create box
nimbus box ls --section "school"                # List boxes in section
nimbus ls /school/fall-2025:/                   # List box contents
nimbus mkdir /school/fall-2025:/assignments     # Create folder
nimbus upload ./file.zip /school/fall-2025:/assignments # Upload file
nimbus download /school/fall-2025:/assignments/file.zip -o ./file.zip # Download
```

## Environment Variables

Required for local development:
```
PORT=8080
LOCAL_DEV=true
DB_DSN=postgres://nimbus:nimbus@localhost:5432/nimbus?sslmode=disable
AWS_REGION=us-east-1
S3_BUCKET=nimbus-dev
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
S3_ENDPOINT=http://localhost:4566
S3_FORCE_PATH_STYLE=true
```

## Database Schema

Key tables:
- `users` - User accounts
- `sections` - Top-level containers owned by users
- `boxes` - Containers within sections
- `folders` - Hierarchical directories within boxes
- `files` - File metadata with S3 references
- `file_versions` - File version history

Local dev includes seeded user: `00000000-0000-0000-0000-000000000001` with email `local@dev`.

## S3 Key Structure

```
org/<owner-id>/sections/<section-id>/boxes/<box-id>/folders/<folder-id>/files/<file-id>/v<version>/blob
```

## Implementation Notes

- Pre-signed URLs have 5-15 minute TTL
- Upload flow: CLI requests pre-signed URL → uploads directly to S3 → calls complete endpoint
- Download flow: CLI requests pre-signed URL → downloads directly from S3
- Path resolution converts human-readable paths to internal UUIDs
- MVP focuses on local development with stubbed authentication
- Future phases add real OIDC, sharing, audit logs, and advanced features