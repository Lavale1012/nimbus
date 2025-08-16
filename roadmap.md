# Nimbus CLI Roadmap

## Phase 1: MVP (Local Development)

**Goal**: Working end-to-end local development environment with core file operations.

### Infrastructure & Setup
- [ ] Set up Go project structure (`cli/`, `service/`, `infra/`)
- [ ] Create `docker-compose.yml` with PostgreSQL and LocalStack
- [ ] Initialize LocalStack with S3 bucket creation script
- [ ] Set up environment configuration (`.env` files)

### Database Foundation
- [ ] Create initial PostgreSQL migration with core tables:
  - `users`, `sections`, `boxes`, `folders`, `files`, `file_versions`
- [ ] Seed local development user and test data
- [ ] Set up database connection and migration runner

### API Server (Go + Gin)
- [ ] Basic HTTP server setup with health endpoint (`GET /healthz`)
- [ ] Stub authentication middleware (`LOCAL_DEV=true` mode)
- [ ] User endpoint (`GET /users/me`)
- [ ] Section management:
  - `POST /sections` - Create section
  - `GET /sections` - List user sections
- [ ] Box management:
  - `POST /boxes` - Create box in section
  - `GET /sections/:id/boxes` - List boxes
- [ ] Folder operations:
  - `POST /folders` - Create folder
  - `GET /boxes/:id/list` - List box root contents
  - `GET /folders/:id/list` - List folder contents
- [ ] Path resolution:
  - `GET /resolve?path=/Section/Box:/folder/path` - Resolve to IDs
- [ ] File upload flow:
  - `POST /files/presign-upload` - Generate pre-signed PUT URL
  - `POST /files/:id/complete` - Finalize upload
- [ ] File download:
  - `GET /files/:id/presign-download` - Generate pre-signed GET URL

### S3 Integration
- [ ] S3 client setup with LocalStack configuration
- [ ] Pre-signed URL generation for uploads (PUT)
- [ ] Pre-signed URL generation for downloads (GET)
- [ ] S3 key structure implementation
- [ ] Upload completion validation (etag, size verification)

### CLI Application (Go + Cobra)
- [ ] CLI project setup with Cobra framework
- [ ] Configuration and API client setup
- [ ] Core commands:
  - `nimbus whoami` - Display current user
  - `nimbus section create <name>` - Create section
  - `nimbus section ls` - List sections
  - `nimbus box create <name> --section <section>` - Create box
  - `nimbus box ls --section <section>` - List boxes
  - `nimbus ls <path>` - List contents at path
  - `nimbus mkdir <path>` - Create folder
  - `nimbus upload <file> <path>` - Upload file
  - `nimbus download <path> -o <output>` - Download file
- [ ] Path parsing and validation (`/Section/Box:/folder/path` format)
- [ ] HTTP client for API communication
- [ ] File transfer logic for pre-signed URLs
- [ ] Progress indicators for uploads/downloads

### Testing & Validation
- [ ] Unit tests for core business logic
- [ ] Integration tests for API endpoints
- [ ] End-to-end tests for CLI workflows
- [ ] Documentation for local development setup

### MVP Acceptance Criteria
1. ✅ `docker compose up -d` starts all services successfully
2. ✅ API responds to health checks and user info
3. ✅ Can create sections and boxes via CLI
4. ✅ Can upload a file and verify it's stored in S3 with correct metadata
5. ✅ Can download the same file and verify byte-for-byte integrity
6. ✅ Path resolution works for all CLI operations
7. ✅ All CLI commands execute without errors for happy path scenarios

---

## Phase 2: Authentication & Authorization

**Goal**: Replace stub auth with real OIDC and implement RBAC.

### Authentication
- [ ] OIDC integration (Auth0/Cognito/Clerk)
- [ ] Device flow for CLI authentication
- [ ] JWT validation in API middleware
- [ ] Token storage in CLI (keychain/secure storage)
- [ ] Login/logout commands (`nimbus login`, `nimbus logout`)

### Authorization (RBAC)
- [ ] Permission model design (owner, editor, viewer)
- [ ] Authorization middleware for API endpoints
- [ ] Resource ownership validation
- [ ] Permission inheritance (section → box → folder → file)

### Multi-user Support
- [ ] User registration/management
- [ ] User isolation in database queries
- [ ] S3 key structure with user prefixes
- [ ] API rate limiting per user

---

## Phase 3: Sharing & Collaboration

**Goal**: Enable secure sharing of files and folders between users.

### Sharing System
- [ ] Sharing database schema (`shares` table)
- [ ] Share creation and management APIs
- [ ] Link-based sharing (public URLs with expiration)
- [ ] User-based sharing (invite by email)
- [ ] Permission levels (read, write, admin)

### CLI Sharing Commands
- [ ] `nimbus share create <path> --user <email> --permission <level>`
- [ ] `nimbus share ls <path>` - List active shares
- [ ] `nimbus share revoke <share-id>`
- [ ] `nimbus share link <path> --expires <duration>`

### Security
- [ ] Share link generation and validation
- [ ] Permission enforcement in all operations
- [ ] Audit logging for share activities
- [ ] Share expiration handling

---

## Phase 4: Advanced Features

**Goal**: Production-ready features for scalability and usability.

### File Operations
- [ ] File versioning and history
- [ ] Soft delete and trash functionality
- [ ] File restoration from versions
- [ ] Bulk operations (upload/download multiple files)
- [ ] Resumable uploads for large files
- [ ] Multipart upload support

### Search & Discovery
- [ ] File search by name and content type
- [ ] Metadata search (tags, creation date, size)
- [ ] Full-text search integration
- [ ] Advanced filtering options

### Storage Management
- [ ] Storage quotas per user/section
- [ ] Usage reporting and analytics
- [ ] Lifecycle policies (archive old files)
- [ ] Duplicate file detection

### Performance & Reliability
- [ ] Caching layer (Redis)
- [ ] Background job processing (file operations)
- [ ] Retry mechanisms for failed uploads
- [ ] Connection pooling and optimization
- [ ] Monitoring and alerting

---

## Phase 5: Enterprise Features

**Goal**: Enterprise-grade security, compliance, and management.

### Security & Compliance
- [ ] Server-side encryption with KMS
- [ ] Compliance reporting (GDPR, SOX)
- [ ] Data retention policies
- [ ] Encryption at rest and in transit
- [ ] Security scanning and virus detection

### Administration
- [ ] Admin dashboard and APIs
- [ ] Organization management
- [ ] User provisioning and deprovisioning
- [ ] Bulk user operations
- [ ] License management

### Audit & Monitoring
- [ ] Comprehensive audit logging
- [ ] Activity monitoring and alerts
- [ ] Performance metrics and dashboards
- [ ] Cost tracking and optimization
- [ ] Compliance reporting

### Integration
- [ ] API for third-party integrations
- [ ] Webhook support for events
- [ ] SAML/SSO integration
- [ ] Directory service integration (LDAP/AD)

---

## Development Milestones

| Phase | Duration | Key Deliverable |
|-------|----------|----------------|
| Phase 1 (MVP) | 4-6 weeks | Working local development environment |
| Phase 2 | 2-3 weeks | Real authentication and multi-user support |
| Phase 3 | 3-4 weeks | Sharing and collaboration features |
| Phase 4 | 6-8 weeks | Production-ready advanced features |
| Phase 5 | 8-12 weeks | Enterprise-grade platform |

## Success Metrics

### MVP Success
- All acceptance criteria met
- Upload/download operations complete successfully
- Local development environment is stable
- Core CLI commands work reliably

### Production Success
- Support for 1000+ concurrent users
- 99.9% uptime SLA
- Sub-second response times for API calls
- Secure file sharing with audit trails
- Enterprise security compliance