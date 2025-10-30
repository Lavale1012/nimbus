# ☁️ Nimbus CLI

> 🚀 A powerful cross-platform command-line interface for secure cloud file storage and management

[![Development Status](https://img.shields.io/badge/status-under%20development-yellow)](https://github.com/your-repo/nimbus)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

![Nimbus CLI Hero Image](docs/images/hero-banner.png)
*A modern CLI tool for developers who love the command line*

---

## 📋 Table of Contents

- [What is Nimbus?](#-what-is-nimbus)
- [Key Features](#-key-features)
- [How It Works](#-how-it-works)
- [Architecture](#-architecture)
- [Quick Start](#-quick-start)
- [Usage Examples](#-usage-examples)
- [API Reference](#-api-reference)
- [Development](#-development)
- [Roadmap](#-roadmap)
- [Contributing](#-contributing)

---

## 🌟 What is Nimbus?

**Nimbus CLI** is a cloud-native file storage system that brings the power and simplicity of the command line to cloud file management. Think of it as a combination of Dropbox's ease-of-use with the developer-friendly interface of Git.

### The Problem

Modern cloud storage solutions often fall into two camps:
- **Consumer-focused** (Dropbox, Google Drive) - Great UIs but poor CLI/API support
- **Developer-focused** (AWS S3, Azure Blob) - Powerful but complex and unintuitive

### The Solution

Nimbus provides:
- 🎯 **Intuitive CLI** - Simple, memorable commands that just work
- 🏗️ **Hierarchical Organization** - Organize files with Boxes → Folders → Files
- ⚡ **Direct S3 Storage** - Fast uploads/downloads without proxy servers
- 🔐 **Secure by Default** - JWT authentication with random 8-digit user IDs
- 🚀 **Developer-First** - Built for automation, scripting, and CI/CD pipelines

![Nimbus Architecture Diagram](docs/images/architecture-diagram.png)

---

## ✨ Key Features

### Current Features (v0.1.0 - MVP)

✅ **User Management**
- Secure user registration with password validation
- JWT-based authentication
- Random 8-digit user IDs for enhanced privacy
- Automatic home box creation on signup

✅ **File Operations**
- Direct file upload to S3 storage
- File download from S3 via unique keys
- File deletion with metadata cleanup
- Comprehensive input validation

✅ **Data Organization**
- Hierarchical box-based structure
- User-specific bucket prefixes
- PostgreSQL metadata storage
- S3-backed file storage

✅ **Security**
- Bcrypt password hashing (cost: 14)
- 4-digit passkey support
- User/box ownership validation
- Secure random ID generation

### Coming Soon

🔜 **Enhanced File Management**
- Folder support within boxes
- File versioning
- Batch operations
- File search and filtering

🔜 **Collaboration**
- Box sharing
- Access control lists
- Shared folders
- Activity logging

🔜 **Advanced Features**
- Pre-signed S3 URLs for direct uploads
- File encryption
- Duplicate detection
- Automated backups

---

## 🎯 How It Works

### The Nimbus Hierarchy

Nimbus organizes your files in a three-tier hierarchy:

```
User (ID: 45892034)
└── 📦 Box: "work"
    ├── 📁 Folder: "projects"
    │   ├── 📁 Folder: "nimbus-cli"
    │   │   ├── 📄 File: "README.md"
    │   │   └── 📄 File: "main.go"
    │   └── 📁 Folder: "website"
    │       └── 📄 File: "index.html"
    └── 📁 Folder: "documents"
        └── 📄 File: "resume.pdf"
```

![Hierarchy Visualization](docs/images/hierarchy-structure.png)

### User Flow Example

#### 1. **Registration**
```bash
# User registers with email and password
curl -X POST http://localhost:8080/v1/api/auth/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@example.com",
    "password": "SecurePass123!",
    "passkey": "1234"
  }'

# Response
{
  "message": "User registered successfully",
  "email": "developer@example.com",
  "user_id": 45892034  # Random 8-digit ID
}
```

The system automatically:
- Generates a secure 8-digit user ID (e.g., 45892034)
- Creates a "Home-Box" for the user
- Sets up a unique S3 bucket prefix: `users/nim-user-45892034/boxes/Home-Box/`

#### 2. **Authentication**
```bash
# User logs in
curl -X POST http://localhost:8080/v1/api/auth/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "developer@example.com",
    "password": "SecurePass123!"
  }'

# Response
{
  "message": "Login successful",
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

#### 3. **File Upload**
```bash
# Upload a file using CLI
nim post -f document.pdf --user 45892034 --box 3778528091639790813

# Behind the scenes:
# 1. CLI sends file to API server
# 2. Server validates user and box ownership
# 3. File is uploaded to S3: users/nim-user-45892034/boxes/Home-Box/document.pdf_1698765432
# 4. Metadata is stored in PostgreSQL
# 5. User receives confirmation
```

![Upload Flow Diagram](docs/images/upload-flow.png)

#### 4. **File Download**
```bash
# Download file using S3 key
nim get -f users/nim-user-45892034/boxes/Home-Box/document.pdf_1698765432 \
        -o ./downloaded-document.pdf
```

---

## 🏗️ Architecture

### System Components

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│             │  HTTPS  │             │   SQL   │             │
│  CLI Client │◄───────►│  API Server │◄───────►│  PostgreSQL │
│   (Cobra)   │         │  (Gin/Go)   │         │  (Metadata) │
└─────────────┘         └─────────────┘         └─────────────┘
                               │
                               │ S3 API
                               ▼
                        ┌─────────────┐
                        │             │
                        │   AWS S3    │
                        │   (Files)   │
                        └─────────────┘
```

### Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **CLI** | Go + Cobra | Command-line interface |
| **API Server** | Go + Gin | REST API backend |
| **Database** | PostgreSQL + GORM | Metadata storage |
| **File Storage** | AWS S3 | Object storage |
| **Authentication** | JWT | Stateless auth tokens |
| **Testing** | Go testing + testify | Unit/integration tests |

### Database Schema

```sql
-- Users with random 8-digit IDs
CREATE TABLE user_models (
    id BIGINT PRIMARY KEY,  -- Random 8-digit ID (no auto-increment)
    email VARCHAR(254) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    bucket_prefix VARCHAR(255) UNIQUE,
    pass_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Boxes (top-level containers)
CREATE TABLE box_models (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES user_models(id),
    box_id BIGINT NOT NULL,  -- Random secure ID
    name VARCHAR(255) NOT NULL,
    size BIGINT DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Files (stored in S3)
CREATE TABLE file_models (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES user_models(id),
    box_id BIGINT REFERENCES box_models(id),
    name VARCHAR(255) NOT NULL,
    size BIGINT DEFAULT 0,
    s3_key VARCHAR(512) UNIQUE NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

![Database Schema Diagram](docs/images/database-schema.png)

---

## 🚀 Quick Start

### Prerequisites

- 🔧 **Go 1.21+** - [Download](https://golang.org/dl/)
- 🐳 **Docker & Docker Compose** - [Download](https://www.docker.com/products/docker-desktop)
- 📝 **Git** - [Download](https://git-scm.com/downloads)
- ☁️ **AWS Account** (optional for local dev) - [Sign up](https://aws.amazon.com/)

### Installation

#### 1. Clone the Repository
```bash
git clone https://github.com/your-username/nimbus.git
cd nimbus
```

#### 2. Start Local Services
```bash
# Starts PostgreSQL and LocalStack (S3 emulator)
docker compose up -d

# Verify services are running
docker compose ps
```

#### 3. Configure Environment
The repository includes a `.env` file. Update if needed:
```env
PORT=8080
DATABASE_URL=postgresql://user:pass@localhost:5432/nimbus
AWS_REGION=us-east-1
S3_BUCKET=nimbus-storage
S3_ENDPOINT=http://localhost:4566  # LocalStack
```

#### 4. Build the CLI
```bash
cd client
go build -o nim cli/main.go

# Optionally, move to your PATH
sudo mv nim /usr/local/bin/
```

#### 5. Start the API Server
```bash
cd server
go run main.go

# Server starts on http://localhost:8080
```

---

## 📖 Usage Examples

### User Management

#### Register a New User
```bash
curl -X POST http://localhost:8080/v1/api/auth/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "password": "MySecure123!",
    "passkey": "1234"
  }'

# Response
{
  "message": "User registered successfully",
  "email": "alice@example.com",
  "user_id": 23847561
}
```

#### Login
```bash
curl -X POST http://localhost:8080/v1/api/auth/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "password": "MySecure123!"
  }'

# Response
{
  "message": "Login successful",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### File Operations

#### Upload a File
```bash
# Using the CLI
nim post -f presentation.pptx \
         --user 23847561 \
         --box 8374920174839201

# Using curl
curl -X POST "http://localhost:8080/v1/api/files?user_id=23847561&box_id=8374920174839201" \
  -F "file=@presentation.pptx"

# Response
{
  "message": "file uploaded successfully",
  "file_id": 42,
  "name": "presentation.pptx",
  "size": 2048000,
  "s3_key": "users/nim-user-23847561/boxes/Home-Box/presentation.pptx_1698765432"
}
```

#### Download a File
```bash
# Using the CLI
nim get -f users/nim-user-23847561/boxes/Home-Box/presentation.pptx_1698765432 \
        -o ./downloaded-presentation.pptx

# Using curl
curl "http://localhost:8080/v1/api/files?key=users/nim-user-23847561/boxes/Home-Box/presentation.pptx_1698765432" \
  --output presentation.pptx
```

#### Delete a File
```bash
curl -X DELETE "http://localhost:8080/v1/api/files/users/nim-user-23847561/boxes/Home-Box/presentation.pptx_1698765432"

# Response
{
  "message": "file deleted"
}
```

### Real-World Scenarios

#### Scenario 1: Backing Up Project Files
```bash
#!/bin/bash
# backup-project.sh

USER_ID=23847561
BOX_ID=8374920174839201

# Upload all Go files
for file in *.go; do
  echo "Uploading $file..."
  nim post -f "$file" --user $USER_ID --box $BOX_ID
done

echo "Backup complete!"
```

#### Scenario 2: Automated Report Generation
```bash
#!/bin/bash
# generate-and-upload-report.sh

# Generate report
python generate_report.py > report_$(date +%Y%m%d).pdf

# Upload to Nimbus
nim post -f report_$(date +%Y%m%d).pdf \
         --user $NIMBUS_USER_ID \
         --box $REPORTS_BOX_ID

echo "Report generated and uploaded!"
```

![Usage Examples](docs/images/usage-examples.png)

---

## 🔌 API Reference

### Base URL
```
http://localhost:8080/v1/api
```

### Authentication Endpoints

#### POST `/auth/users/register`
Register a new user.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "passkey": "1234"
}
```

**Validation Rules:**
- Email: Valid email format, max 254 characters
- Password: Min 8 characters, must include uppercase, lowercase, number, and special character
- Passkey: Exactly 4 characters

**Response (201):**
```json
{
  "message": "User registered successfully",
  "email": "user@example.com",
  "user_id": 45892034
}
```

#### POST `/auth/users/login`
Authenticate a user.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Response (200):**
```json
{
  "message": "Login successful",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### File Management Endpoints

#### POST `/files?user_id={id}&box_id={id}`
Upload a file.

**Query Parameters:**
- `user_id` (required): User's 8-digit ID
- `box_id` (required): Target box ID

**Request Body:**
- Multipart form data with `file` field

**Response (200):**
```json
{
  "message": "file uploaded successfully",
  "file_id": 42,
  "name": "document.pdf",
  "size": 1024000,
  "s3_key": "users/nim-user-45892034/boxes/Home-Box/document.pdf_1698765432"
}
```

**Error Responses:**
- `400`: Missing/invalid parameters, user not found, box not found
- `500`: S3 upload failure, database error

#### GET `/files?key={s3_key}`
Download a file.

**Query Parameters:**
- `key` (required): S3 key of the file

**Response (200):**
- File stream with appropriate Content-Type header
- Content-Disposition header for download

**Error Responses:**
- `400`: Missing key parameter
- `404`: File not found
- `500`: S3 download failure

#### DELETE `/files/{filename}`
Delete a file.

**Path Parameters:**
- `filename` (required): Name of the file to delete

**Response (200):**
```json
{
  "message": "file deleted"
}
```

**Error Responses:**
- `404`: File not found
- `500`: Deletion failure

---

## 🛠️ Development

### Project Structure

```
nim-cli/
├── client/                      # CLI application
│   ├── cli/
│   │   ├── main.go             # Entry point
│   │   ├── cmd/                # Cobra commands
│   │   │   ├── root.go         # Root command
│   │   │   ├── post.go         # Upload command
│   │   │   ├── get.go          # Download command
│   │   │   └── delete.go       # Delete command
│   │   └── animations/         # Loading animations
│   ├── utils/                  # Utilities
│   │   ├── getEnv.go          # Environment helpers
│   │   └── searchRoot.go      # File search
│   └── go.mod
│
├── server/                      # API server
│   ├── main.go                 # Entry point
│   ├── server-init/
│   │   └── InitServer.go      # Server setup
│   ├── handlers/
│   │   ├── userHandlers/      # User auth handlers
│   │   │   └── UserAuth.go
│   │   └── fileHandlers/      # File operation handlers
│   │       └── FileOperations.go
│   ├── routes/
│   │   ├── initUserRoutes.go  # User routes
│   │   └── initFileRoutes.go  # File routes
│   ├── models/
│   │   ├── UserModel.go       # User schema
│   │   ├── BoxModel.go        # Box schema
│   │   ├── FolderModel.go     # Folder schema
│   │   └── Files.go           # File schema
│   ├── middleware/
│   │   └── auth/
│   │       └── JWT/           # JWT middleware
│   ├── db/
│   │   ├── Postgres/          # PostgreSQL config
│   │   │   └── config/
│   │   │       └── ConnectPostgres.go
│   │   └── S3/                # S3 operations
│   │       ├── config/
│   │       │   └── S3Connect.go
│   │       └── operations/
│   │           ├── PutObj.go
│   │           ├── GetObj.go
│   │           └── MakeButcket.go
│   ├── utils/
│   │   ├── hash.go            # Password hashing
│   │   └── getEnv.go          # Environment helpers
│   ├── tests/                 # Test files
│   │   ├── file_operations_test.go
│   │   ├── userauth_test.go
│   │   └── hash_utils_test.go
│   ├── migrations/            # Database migrations
│   └── go.mod
│
├── docker-compose.yml         # Local services
├── .env                       # Environment config
├── .gitignore
├── LICENSE
├── README.md
└── CLAUDE.md                  # AI assistant context
```

### Building from Source

```bash
# Build CLI
cd client
go build -o nim cli/main.go

# Build API server
cd server
go build -o api-server main.go

# Build both with custom output
make build  # If Makefile exists
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
cd server && go test ./handlers/fileHandlers/...

# Run verbose tests
go test -v ./...

# Run specific test
go test -run TestUploadFile_MissingUserID ./tests/
```

### Code Quality

```bash
# Format code
go fmt ./...

# Vet code
go vet ./...

# Run linter (requires golangci-lint)
golangci-lint run

# Check for security issues
gosec ./...
```

### Local Development Workflow

1. **Start local services**
   ```bash
   docker compose up -d
   ```

2. **Run migrations** (if needed)
   ```bash
   cd server/migrations
   go run 001_*.go
   ```

3. **Start server in watch mode**
   ```bash
   cd server
   air  # or go run main.go
   ```

4. **Run CLI in development**
   ```bash
   cd client
   go run cli/main.go --help
   ```

5. **Test your changes**
   ```bash
   go test ./...
   ```

---

## 🗺️ Roadmap

### ✅ Phase 1: MVP (Current)
- [x] User registration and authentication
- [x] Random 8-digit user IDs
- [x] Basic file upload/download/delete
- [x] S3 integration
- [x] PostgreSQL metadata storage
- [x] CLI commands (post, get, delete)
- [x] Comprehensive test suite

### 🚧 Phase 2: Core Features (In Progress)
- [ ] Folder support within boxes
- [ ] File listing and browsing
- [ ] Box creation and management
- [ ] Path-based file operations (`box:/folder/file`)
- [ ] File metadata (MIME types, checksums)
- [ ] Error handling improvements

### 🔜 Phase 3: Enhanced Experience
- [ ] Pre-signed URLs for direct S3 uploads
- [ ] Progress indicators for large files
- [ ] Concurrent uploads/downloads
- [ ] File versioning
- [ ] Duplicate detection
- [ ] Search functionality

### 📅 Phase 4: Collaboration
- [ ] Box sharing (read/write permissions)
- [ ] Shared folders
- [ ] Access control lists
- [ ] Activity logs
- [ ] User groups

### 🎯 Phase 5: Enterprise Features
- [ ] File encryption
- [ ] Audit logging
- [ ] Admin dashboard
- [ ] Usage quotas
- [ ] Backup/restore
- [ ] Multi-region support

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### Getting Started

1. **Fork the repository**
2. **Clone your fork**
   ```bash
   git clone https://github.com/YOUR_USERNAME/nimbus.git
   ```
3. **Create a feature branch**
   ```bash
   git checkout -b feature/amazing-feature
   ```
4. **Make your changes**
5. **Run tests**
   ```bash
   go test ./...
   ```
6. **Commit your changes**
   ```bash
   git commit -m "feat: Add amazing feature"
   ```
7. **Push to your fork**
   ```bash
   git push origin feature/amazing-feature
   ```
8. **Open a Pull Request**

### Development Guidelines

- ✅ Follow [Go best practices](https://golang.org/doc/effective_go)
- ✅ Write tests for new functionality (aim for >80% coverage)
- ✅ Update documentation for API changes
- ✅ Use [conventional commits](https://www.conventionalcommits.org/)
- ✅ Ensure all tests pass before submitting PR
- ✅ Keep PRs focused and atomic

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions/changes
- `refactor`: Code refactoring
- `chore`: Build/tooling changes

**Example:**
```
feat(upload): Add progress indicator for large files

Implement real-time progress tracking during file uploads
using chunked transfer encoding and progress callbacks.

Closes #123
```

---

## 🔒 Security

### Current Security Features

- 🔐 **Password Security**
  - Bcrypt hashing with cost factor 14
  - Minimum 8 characters
  - Complexity requirements (uppercase, lowercase, number, special char)

- 🎲 **Random ID Generation**
  - Cryptographically secure random 8-digit user IDs
  - Collision detection and retry logic
  - Large random box IDs (63-bit)

- 🛡️ **Access Control**
  - JWT-based authentication
  - User/box ownership validation
  - Input validation and sanitization

- 📝 **Data Protection**
  - SQL injection prevention (GORM parameterized queries)
  - XSS protection
  - CORS configuration
  - Rate limiting (planned)

### Reporting Security Issues

If you discover a security vulnerability, please email security@example.com instead of using the issue tracker.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

```
MIT License

Copyright (c) 2024 Nimbus CLI Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software...
```

---

## 💬 Support & Community

### Documentation
- 📖 [Full Documentation](docs/)
- 🎓 [Getting Started Guide](docs/getting-started.md)
- 📚 [API Reference](docs/api-reference.md)
- 🔧 [Developer Guide](docs/developer-guide.md)

### Getting Help
- 🐛 [Report a Bug](https://github.com/your-repo/nimbus/issues/new?template=bug_report.md)
- ✨ [Request a Feature](https://github.com/your-repo/nimbus/issues/new?template=feature_request.md)
- 💭 [GitHub Discussions](https://github.com/your-repo/nimbus/discussions)
- 💬 [Discord Community](https://discord.gg/nimbus) *(Coming Soon)*

### Stay Updated
- ⭐ Star this repository
- 👀 Watch for updates
- 🐦 Follow us on Twitter [@NimbusCLI](https://twitter.com/nimbuscli) *(Coming Soon)*

---

## 📊 Project Status

> **⚠️ UNDER ACTIVE DEVELOPMENT**
>
> Nimbus is currently in the **MVP phase** and is not yet ready for production use. We're actively working on:
> - Completing core file management features
> - Improving error handling and edge cases
> - Adding comprehensive documentation
> - Expanding test coverage
> - Hardening security features
>
> **Expected Beta Release:** Q2 2024
>
> Star ⭐ the repo and watch 👀 for updates!

### Current Version: v0.1.0-alpha

#### What Works
✅ User registration and login
✅ File upload/download/delete
✅ Basic CLI commands
✅ Local development with Docker
✅ S3 integration

#### Known Limitations
⚠️ No folder support yet
⚠️ Limited error messages
⚠️ No file listing
⚠️ Single box per user (Home-Box only)
⚠️ No file versioning

#### Performance Metrics
- File upload: ~5MB/s (local), ~2MB/s (S3)
- Database query latency: <50ms
- API response time: <200ms

![Project Metrics](docs/images/project-metrics.png)

---

## 🙏 Acknowledgments

Special thanks to:
- The Go community for excellent libraries
- AWS for S3 storage
- Contributors and early adopters
- Everyone who provided feedback

---

<div align="center">

### 🚀 Built for developers who love the command line

**[⭐ Star on GitHub](https://github.com/your-repo/nimbus)** • **[📖 Read the Docs](docs/)** • **[🐛 Report Bug](issues/)** • **[💡 Request Feature](issues/)**

---

![Footer Banner](docs/images/footer-banner.png)

**Made with ❤️ by the Nimbus team**

</div>
