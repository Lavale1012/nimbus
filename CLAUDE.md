# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Nimbus CLI is a cloud file storage and management system with a Go-based CLI client and REST API server. The system uses a hierarchical organization: Boxes → Folders → Files, with direct S3 storage and PostgreSQL metadata.

## Architecture

The codebase follows a client-server architecture with two main Go modules:

### Client (`/client`)
- **Module**: `github.com/nimbus/cli`  
- **Entry Point**: `client/cli/main.go`
- **CLI Framework**: Cobra for command structure
- **Key Components**:
  - `cli/cmd/` - Cobra command definitions
  - `cli/animations/` - Loading animations
  - `utils/getEnv.go` - Environment variable handling

### Server (`/server`) 
- **Module**: `github.com/nimbus/api`
- **Entry Point**: `server/main.go`
- **Web Framework**: Gin for HTTP routing
- **Key Components**:
  - `server-init/InitServer.go` - Server initialization and routing setup
  - `models/` - GORM data models (Files, Boxes, Users)
  - `handlers/` - HTTP request handlers organized by domain
  - `routes/` - Route initialization separated by domain
  - `db/` - Database connections (PostgreSQL via GORM, S3 via AWS SDK)
  - `utils/getEnv.go` - Environment variable handling

## Development Commands

### Building
```bash
# Build CLI client
cd client && go build -o nimbus cli/main.go

# Build API server  
cd server && go build -o api-server main.go

# Build from root (if Makefile exists)
make build
```

### Running
```bash
# Start API server
cd server && go run main.go

# Run CLI commands
cd client && go run cli/main.go [command]

# Start local services (PostgreSQL + LocalStack S3)
docker compose up -d
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific module tests
cd client && go test ./...
cd server && go test ./...
```

### Code Quality
```bash
# Format code
go fmt ./...

# Run linter (if golangci-lint installed)
golangci-lint run
```

## Key Environment Variables

Environment configuration is loaded from `.env` file:

- `PORT` - API server port (default: 8080)
- `LOCAL_DEV` - Enable local development mode
- `DB_DSN` - PostgreSQL connection string
- `AWS_REGION` - AWS region for S3
- `S3_BUCKET` - S3 bucket name
- `S3_ENDPOINT` - S3 endpoint (use `http://localhost:4566` for LocalStack)
- `S3_FORCE_PATH_STYLE` - Use path-style S3 URLs (required for LocalStack)
- `DEFAULT_UPLOAD_PATH` - CLI upload endpoint
- `DEFAULT_DOWNLOAD_URL` - CLI download endpoint

## Project Structure Patterns

### Module Organization
- Each Go module (`client/`, `server/`) has its own `go.mod`
- Import paths use GitHub-style module names but point to local directories
- Server imports: `github.com/nimbus/api/[package]`
- Client imports: `github.com/nimbus/cli/[package]`

### File Organization  
- Domain-driven structure: handlers, routes, and models organized by feature area
- Separate initialization files (`InitServer.go`, route initialization files)
- Utility functions in dedicated `utils/` packages
- Configuration and connection logic in `config/` subdirectories

### Current Implementation Status
The codebase implements basic file upload/download functionality but differs from the README's comprehensive CLI description. Current CLI commands:
- `post --file [path]` - Upload file to server
- `get --file [key] --output [path]` - Download file from S3

The full hierarchical path system (`box:/folder/file`) described in README is not yet implemented.