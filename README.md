# ☁️ Nimbus CLI

> 🚀 A powerful cross-platform command-line interface for cloud file storage and management

## 📋 Overview

Nimbus CLI provides a hierarchical file organization system with direct S3 storage and a powerful command-line interface. Files are organized as:

- 📦 **Boxes** → Top-level containers (e.g., "work", "school", "photos")
- 📁 **Folders** → Hierarchical directories within a box
- 📄 **Files** → Versioned objects with direct S3 storage

## 🏗️ Architecture

- 💻 **CLI**: Go + Cobra for command-line interface
- 🌐 **API**: Go + Gin for REST API server
- 🗄️ **Database**: PostgreSQL for metadata storage
- 🪣 **Storage**: AWS S3 with pre-signed URLs (direct client uploads/downloads)
- 🔐 **Auth**: OIDC (Auth0/Cognito/Clerk) with RBAC

## 🚀 Quick Start

### 📋 Prerequisites

- 🔧 Go 1.21+
- 🐳 Docker & Docker Compose
- 📝 Git

### 🛠️ Local Development Setup

1. **📥 Clone the repository**
   ```bash
   git clone <repository-url>
   cd nim-cli
   ```

2. **🚀 Start local services**
   ```bash
   docker compose up -d
   ```
   This starts PostgreSQL and LocalStack (S3 emulator).

3. **⚙️ Environment is already configured**
   The repository includes a `.env` file with local development settings.

4. **🔨 Build and install CLI**
   ```bash
   cd client && go build -o nim cli/main.go
   ./nim --help
   ```

5. **▶️ Start API server**
   ```bash
   cd server && go run main.go
   ```

## 📖 Usage

### ✅ Current Commands (MVP Implementation)

```bash
# 📤 Upload a file
nim post --file ./notes.pdf

# 📥 Download a file
nim get --file <s3-key> --output ./downloaded-notes.pdf

# 🗑️ Delete a file
nim del --file <filename>
```

### 🔮 Planned Commands (Future Implementation)

```bash
# 👤 Check current user
nim whoami

# 📦 Create a box
nim box create "school"

# 📋 List boxes
nim box ls

# 📁 List contents of a box
nim ls school:/

# 🆕 Create a folder
nim mkdir school:/assignments

# 📤 Upload a file
nim upload ./notes.pdf school:/assignments

# 📥 Download a file
nim download school:/assignments/notes.pdf -o ./downloaded-notes.pdf

# 🗑️ Remove a file
nim rm school:/assignments/notes.pdf
```

### 🗂️ Path Format

Nimbus uses a hierarchical path format:
```
📦 BoxName:/📁folder/📁subfolder/📄file.ext
```

- The `:` separates the box from the folder path
- Folder paths use standard `/` separators

## 🛠️ Development

### 📁 Project Structure

```
nim-cli/
├── client/                   # CLI application
│   ├── cli/
│   │   ├── main.go          # CLI entry point
│   │   ├── cmd/             # Cobra commands
│   │   └── animations/      # Loading animations
│   ├── utils/               # CLI utilities
│   └── go.mod               # CLI module
├── server/                   # API server
│   ├── main.go              # Server entry point
│   ├── server-init/         # Server initialization
│   ├── handlers/            # HTTP handlers
│   ├── routes/              # Route definitions
│   ├── models/              # GORM data models
│   ├── db/                  # Database connections
│   │   ├── Postgres/        # PostgreSQL config
│   │   └── S3/              # S3 operations
│   ├── utils/               # Server utilities
│   └── go.mod               # Server module
├── docker-compose.yml       # Local development
└── .env                     # Environment configuration
```

### 🔨 Building

```bash
# 💻 Build CLI
cd client && go build -o nim cli/main.go

# 🌐 Build API server
cd server && go build -o api-server main.go
```

### 🧪 Testing

```bash
# 🚀 Run all tests
go test ./...

# 📊 Run tests with coverage
go test -cover ./...

# 🎯 Run tests for specific modules
cd client && go test ./...
cd server && go test ./...
```

### ✨ Code Quality

```bash
# 🎨 Format code
go fmt ./...

# 🔍 Run linter (if golangci-lint is installed)
golangci-lint run
```

## ⚙️ Configuration

### 🌍 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | API server port | `8080` |
| `LOCAL_DEV` | Enable local development mode | `false` |
| `DB_DSN` | PostgreSQL connection string | Required |
| `AWS_REGION` | AWS region | `us-east-1` |
| `S3_BUCKET` | S3 bucket name | Required |
| `S3_ENDPOINT` | S3 endpoint (for LocalStack) | AWS default |
| `S3_FORCE_PATH_STYLE` | Force path-style S3 URLs | `false` |

### 🧪 Local Development

The repository includes a `.env` file with these local development settings:

```env
PORT=8080
LOCAL_DEV=true
DB_DSN=postgres://nimbus:nimbus@localhost:5432/nimbus?sslmode=disable
AWS_REGION=us-east-2
S3_BUCKET=nimbus-cli-storage
S3_ENDPOINT=http://localhost:4566
S3_FORCE_PATH_STYLE=true
DEFAULT_UPLOAD_PATH=http://localhost:8080/v1/api/files
DEFAULT_DOWNLOAD_URL=http://localhost:8080/v1/api/files
```

## 🌐 API Reference

### 🔗 Base URL
```
http://localhost:8080
```

### ✅ Current Endpoints (MVP Implementation)

- `📤 POST /v1/api/files` - Upload file
- `📥 GET /v1/api/files?key=<s3-key>` - Download file
- `🗑️ DELETE /v1/api/files/{filename}` - Delete file

### 🔮 Planned Endpoints (Future Implementation)

- `GET /healthz` - Health check
- `GET /users/me` - Current user info  
- `POST /boxes` - Create box
- `GET /boxes` - List boxes
- `POST /files/presign-upload` - Get upload URL
- `POST /files/:id/complete` - Complete upload
- `GET /files/:id/presign-download` - Get download URL
- `GET /resolve?path=...` - Resolve path to IDs

## 🗺️ Roadmap

See [roadmap.md](roadmap.md) for detailed development phases:

- **🎯 Phase 1 (MVP)**: Local development with core file operations
- **🔐 Phase 2**: Authentication and authorization
- **🤝 Phase 3**: Sharing and collaboration
- **⚡ Phase 4**: Advanced features (search, versioning, bulk ops)
- **🏢 Phase 5**: Enterprise features (compliance, admin tools)

## 🤝 Contributing

1. 🍴 Fork the repository
2. 🌿 Create a feature branch (`git checkout -b feature/amazing-feature`)
3. 💾 Commit your changes (`git commit -m 'Add amazing feature'`)
4. 📤 Push to the branch (`git push origin feature/amazing-feature`)
5. 🔄 Open a Pull Request

### 📝 Development Guidelines

- ✅ Follow Go best practices and idioms
- 🧪 Write tests for new functionality
- 📚 Update documentation for API changes
- 📋 Use conventional commit messages
- 🔍 Ensure all tests pass before submitting PR

## 🔒 Security

- 🔗 All file uploads/downloads use pre-signed S3 URLs (no data flows through API)
- 🎫 JWT-based authentication with OIDC providers
- 🛡️ RBAC for resource access control
- 📝 Audit logging for all operations
- 🔐 Encryption in transit and at rest

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 💬 Support

- 📖 **Documentation**: See the [docs/](docs/) directory
- 🐛 **Issues**: Report bugs and feature requests via GitHub Issues
- 💭 **Discussions**: Use GitHub Discussions for questions and ideas

## 📊 Status

🚧 **Currently in development** - MVP phase targeting core file operations with local development environment.

---

<div align="center">

**Made with ❤️ for developers who love the command line**

</div>