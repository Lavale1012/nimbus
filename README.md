# Nimbus CLI

A cross-platform command-line interface for cloud file storage and management.

## Overview

Nimbus CLI provides a hierarchical file organization system with direct S3 storage and a powerful command-line interface. Files are organized as:

- **Sections** → Top-level containers (e.g., "work", "school", "personal")
- **Boxes** → Containers within a section (e.g., "fall-2025", "photos") 
- **Folders** → Hierarchical directories within a box
- **Files** → Versioned objects with direct S3 storage

## Architecture

- **CLI**: Go + Cobra for command-line interface
- **API**: Go + Gin for REST API server
- **Database**: PostgreSQL for metadata storage
- **Storage**: AWS S3 with pre-signed URLs (direct client uploads/downloads)
- **Auth**: OIDC (Auth0/Cognito/Clerk) with RBAC

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Git

### Local Development Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd nim-cli
   ```

2. **Start local services**
   ```bash
   docker compose up -d
   ```
   This starts PostgreSQL and LocalStack (S3 emulator).

3. **Set up environment**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run database migrations**
   ```bash
   go run ./service/migrations migrate up
   ```

5. **Build and install CLI**
   ```bash
   go build -o nimbus ./cli/cmd/nimbus
   ./nimbus --help
   ```

6. **Start API server**
   ```bash
   go run ./service/cmd/api
   ```

## Usage

### Basic Commands

```bash
# Check current user
nimbus whoami

# Create a section
nimbus section create "school"

# List sections
nimbus section ls

# Create a box within a section
nimbus box create "fall-2025" --section "school"

# List boxes in a section
nimbus box ls --section "school"

# List contents of a box
nimbus ls /school/fall-2025:/

# Create a folder
nimbus mkdir /school/fall-2025:/assignments

# Upload a file
nimbus upload ./notes.pdf /school/fall-2025:/assignments

# Download a file
nimbus download /school/fall-2025:/assignments/notes.pdf -o ./downloaded-notes.pdf

# Remove a file
nimbus rm /school/fall-2025:/assignments/notes.pdf
```

### Path Format

Nimbus uses a hierarchical path format:
```
/SectionName/BoxName:/folder/subfolder/file.ext
```

- Sections and boxes are separated by `/`
- The `:` separates the box from the folder path
- Folder paths use standard `/` separators

## Development

### Project Structure

```
nimbus/
├── cli/                      # CLI application
│   ├── cmd/nimbus/          # Cobra commands
│   ├── internal/api/        # API client
│   ├── internal/auth/       # Authentication
│   ├── internal/transfer/   # File upload/download
│   └── internal/path/       # Path parsing
├── service/                  # API server
│   ├── cmd/api/             # Server entry point
│   ├── internal/httpserver/ # HTTP handlers
│   ├── internal/auth/       # JWT validation
│   ├── internal/storage/    # S3 integration
│   ├── internal/meta/       # Database layer
│   ├── pkg/types/           # Shared DTOs
│   └── migrations/          # Database migrations
├── infra/                   # Infrastructure
│   ├── docker-compose.yml   # Local development
│   └── localstack-init/     # S3 bucket setup
└── docs/                    # Documentation
```

### Building

```bash
# Build CLI
go build -o nimbus ./cli/cmd/nimbus

# Build API server
go build -o api-server ./service/cmd/api

# Build both
make build
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./cli/internal/api
go test ./service/internal/meta
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter (if golangci-lint is installed)
golangci-lint run

# Run tests and linting
make check
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | API server port | `8080` |
| `LOCAL_DEV` | Enable local development mode | `false` |
| `DB_DSN` | PostgreSQL connection string | Required |
| `AWS_REGION` | AWS region | `us-east-1` |
| `S3_BUCKET` | S3 bucket name | Required |
| `S3_ENDPOINT` | S3 endpoint (for LocalStack) | AWS default |
| `S3_FORCE_PATH_STYLE` | Force path-style S3 URLs | `false` |

### Local Development

For local development, use these settings in `.env`:

```env
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

## API Reference

### Base URL
```
http://localhost:8080/v1
```

### Key Endpoints

- `GET /healthz` - Health check
- `GET /users/me` - Current user info
- `POST /sections` - Create section
- `GET /sections` - List sections
- `POST /boxes` - Create box
- `GET /sections/:id/boxes` - List boxes
- `POST /files/presign-upload` - Get upload URL
- `POST /files/:id/complete` - Complete upload
- `GET /files/:id/presign-download` - Get download URL
- `GET /resolve?path=...` - Resolve path to IDs

See [API Documentation](docs/api.md) for full details.

## Roadmap

See [roadmap.md](roadmap.md) for detailed development phases:

- **Phase 1 (MVP)**: Local development with core file operations
- **Phase 2**: Authentication and authorization
- **Phase 3**: Sharing and collaboration
- **Phase 4**: Advanced features (search, versioning, bulk ops)
- **Phase 5**: Enterprise features (compliance, admin tools)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Write tests for new functionality
- Update documentation for API changes
- Use conventional commit messages
- Ensure all tests pass before submitting PR

## Security

- All file uploads/downloads use pre-signed S3 URLs (no data flows through API)
- JWT-based authentication with OIDC providers
- RBAC for resource access control
- Audit logging for all operations
- Encryption in transit and at rest

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: See the [docs/](docs/) directory
- **Issues**: Report bugs and feature requests via GitHub Issues
- **Discussions**: Use GitHub Discussions for questions and ideas

## Status

🚧 **Currently in development** - MVP phase targeting core file operations with local development environment.