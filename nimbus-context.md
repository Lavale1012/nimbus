
# Nimbus CLI — Project Context & Initial Implementation Scope (with Sections & Boxes)

Nimbus CLI is a cross-platform **command-line interface** for cloud file storage and management written in **Go**. Data is organized hierarchically as:

- **Sections** → top-level containers (e.g., “work”, “school”, “personal”)
- **Boxes** → containers within a section (e.g., “fall-2025”, “photos”)
- **Folders** → hierarchical directories within a box
- **Files** → leaf objects, optionally versioned

Large file bytes flow **directly to S3** using **pre-signed URLs**; the API handles authentication, authorization, and metadata only.

---

## Core Tech Stack

- **Go** for the CLI and Backend API (fast, single-binary deployment, great tooling)
- **AWS S3** for durable, scalable object storage (multipart uploads, lifecycle)
- **PostgreSQL** for metadata (sections, boxes, folders, files, versions, shares)
- **OIDC (Auth0/Cognito/Clerk)** for authentication (JWT), **RBAC** for authorization
- **Pre-signed S3 URLs** (PUT/GET) so the CLI never proxies file bytes via the API

> Local development uses **Docker Compose** with Postgres and **LocalStack** (S3).

---

## High-Level Architecture

**CLI (Go + Cobra)** ⇄ **API (Go + Gin)** → **S3 (pre-signed URLs)**  
**Postgres** stores all metadata, ownership, permissions, and version pointers.

### Typical Upload Flow
1. CLI → API: “I want to upload `notes.zip` to `/school/fall-2025:/assignments`”  
   (path resolves to `{section, box, folder}` + intended `name`)
2. API (authZ check) → generates **pre-signed PUT URL** for S3 + returns a `file_id`
3. CLI uses the URL to PUT bytes directly to S3 (multipart if large)
4. CLI → API: complete upload (send `etag`, `size`) → API finalizes metadata/version
5. API emits an audit event (later phase) and returns success

### Typical Download Flow
1. CLI → API: request download for `file-id` (or path `/section/box:/path/to/file`)
2. API (authZ) → returns **pre-signed GET URL** (short TTL)
3. CLI streams bytes to disk

---

## Domain Model

```
Section ──┬── Box ──┬── Folder (tree) ──┬── File (current)
          │         │                   └── FileVersion (1..n)
          │         └── (folders/files live inside a box)
          └── (sections group multiple boxes)
```

### PostgreSQL Tables (MVP)

- **users**: id (uuid), email, created_at  
- **sections**: id (uuid), owner_id (uuid), name, created_at  
- **boxes**: id (uuid), section_id (uuid FK), name, created_at  
- **folders**: id (uuid), box_id (uuid FK), parent_id (nullable), name, created_at  
- **files**: id (uuid), folder_id (uuid FK), owner_id (uuid), name, size, content_type, etag, s3_key, is_deleted, created_at, updated_at  
- **file_versions**: id (uuid), file_id (uuid FK), version_no (int), size, etag, s3_key, created_at  
- **shares** (phase 2+): id, (section_id|box_id|folder_id|file_id), subject, permission, expires_at  
- **audit_logs** (phase 3): id, actor_id, action, object_ref, metadata (jsonb), created_at

> Root **folders** are per **box** (not global). A user can have many **sections**, each with many **boxes**.

---

## Key Naming in S3 (MVP)

```
org/<owner-id>/sections/<section-id>/boxes/<box-id>/folders/<folder-id>/files/<file-id>/v<version>/blob
```

- Pre-signed URL TTL: 5–15 minutes
- Server-side encryption: **SSE-S3** (Phase 1) → **SSE-KMS** (Phase 3+)
- Bucket per environment: `nimbus-dev`, `nimbus-prod`

---

## API Surface (MVP)

**Base URL:** `http://localhost:8080/v1`

- **Health & Me**
  - `GET /healthz` → `{status:"ok"}`
  - `GET /users/me` → `{id, email}` (stubbed user when `LOCAL_DEV=true`)

- **Sections & Boxes**
  - `POST /sections` → create a section `{id, name}`
  - `GET /sections` → list user’s sections
  - `POST /boxes` → create a box `{id, name, section_id}`
  - `GET /sections/:id/boxes` → list boxes in a section

- **Folders**
  - `POST /folders` → `{id, name, box_id, parent_id?}`
  - `GET /boxes/:id/list` → list `{folders, files}` at the box root
  - `GET /folders/:id/list` → list `{folders, files}` inside a folder
  - `GET /resolve?path=/SectionName/BoxName:/folder/sub` → `{section_id, box_id, folder_id}`

- **Files**
  - `POST /files/presign-upload`  
    - Req: `{folder_id, name, size, content_type}`  
    - Res: `{file_id, upload_url, headers, method:"PUT"}` (creates pending file + s3_key)
  - `POST /files/:id/complete`  
    - Req: `{size, etag}` → finalize file + create `file_versions` v1
  - `GET /files/:id/presign-download`  
    - Res: `{download_url, method:"GET"}`

**Auth**  
- `LOCAL_DEV=true`: injects `user_id='00000000-0000-0000-0000-000000000001'`  
- Later: validate JWTs via OIDC (JWKS), apply RBAC to Sections/Boxes/Files

---

## CLI Commands (Nimbus)

```
nimbus login                      # later (OIDC device flow)
nimbus whoami                     # prints current user

# Sections & Boxes
nimbus section create "school"
nimbus section ls
nimbus box create "fall-2025" --section "school"
nimbus box ls --section "school"

# Folder & File Ops (within a box)
nimbus ls /school/fall-2025:/           # box root
nimbus mkdir /school/fall-2025:/assignments
nimbus upload ./notes.zip /school/fall-2025:/assignments
nimbus download /school/fall-2025:/assignments/notes.zip -o ./notes.zip
nimbus rm /school/fall-2025:/assignments/notes.zip
nimbus mv /school/fall-2025:/assignments/notes.zip /school/fall-2025:/archive/notes.zip
```

**Path Semantics**  
- A path begins with `/SectionName/BoxName:/` then a folder path inside that box.  
- CLI resolves names → IDs via `/resolve?path=...`. In scripts you can use IDs directly.

---

## Project Layout

```
nimbus/
  cli/
    cmd/nimbus/                 # cobra commands (whoami, section, box, ls, upload, download)
    internal/auth/              # (phase 2+) device flow, token store (keychain)
    internal/api/               # REST client + DTOs
    internal/transfer/          # HTTP PUT/GET to presigned URLs + progress, retries
    internal/path/              # parse & resolve /Section/Box:/folder paths
  service/
    cmd/api/
    internal/httpserver/        # gin setup, middleware
    internal/auth/              # jwt (stub in local)
    internal/storage/           # s3 presign, key builder
    internal/meta/              # db repo (gorm/sqlc)
    internal/resolve/           # path→ID resolution logic
    pkg/types/                  # shared request/response DTOs
    migrations/
  infra/
    docker-compose.yml          # postgres + localstack
    localstack-init/            # bucket init
  .env.example
  README.md
```

---

## Environment Variables

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

---

## Migration 0001 (Sketch)

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id UUID NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(owner_id, name)
);

CREATE TABLE boxes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  section_id UUID NOT NULL REFERENCES sections(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(section_id, name)
);

CREATE TABLE folders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  box_id UUID NOT NULL REFERENCES boxes(id) ON DELETE CASCADE,
  parent_id UUID REFERENCES folders(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(box_id, parent_id, name)
);

CREATE TABLE files (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  folder_id UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  owner_id UUID NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  size BIGINT,
  content_type TEXT,
  etag TEXT,
  s3_key TEXT UNIQUE NOT NULL,
  is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE file_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
  version_no INT NOT NULL,
  size BIGINT,
  etag TEXT,
  s3_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(file_id, version_no)
);

-- Seed a local dev user
INSERT INTO users (id, email) VALUES
  ('00000000-0000-0000-0000-000000000001', 'local@dev');

-- Seed a default section and box for quickstart (optional)
INSERT INTO sections (id, owner_id, name)
VALUES ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'school');

INSERT INTO boxes (id, section_id, name)
VALUES ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'fall-2025');

-- Create the box root folder
INSERT INTO folders (id, box_id, parent_id, name)
VALUES ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', NULL, '');
```

---

## Initial Implementation Scope (Phase 1 – MVP)

**Goal**: local-dev MVP with the following working end-to-end:

- `nimbus whoami`
- `nimbus section create/list`
- `nimbus box create/list`
- `nimbus ls /Section/Box:/`
- `nimbus mkdir /Section/Box:/path`
- `nimbus upload <file> /Section/Box:/path`
- `nimbus download /Section/Box:/path/file -o <out>`

**Auth**: stubbed (`LOCAL_DEV=true`).  
**Uploads/Downloads**: pre-signed URLs only.  
**Versioning**: create `v1` upon finalize.

### Acceptance Criteria
1. `docker compose up -d` starts Postgres + LocalStack and creates bucket `nimbus-dev`.
2. API starts with `LOCAL_DEV=true` and responds to `/healthz`.
3. CLI can create a **section** and **box**, list them, and resolve a path.
4. Upload of a file to `/Section/Box:/some/folder` succeeds, creates S3 object at computed key, and finalizes metadata (`files`, `file_versions`).
5. Download via path or `file-id` writes identical bytes (size match).

---

## Next Phases (brief)

- **Phase 2**: Real OIDC login + RBAC on Sections/Boxes/Files, sharing (user & link)
- **Phase 3**: Multipart/resumable uploads, audit logs, KMS, lifecycle/quotas
- **Phase 4**: Search (filename), virus-scan/thumbnail workers via SQS, soft delete & retention

---

## Security & IAM (essentials)

- Block Public Access on S3, deny non-TLS
- Principle of least privilege for IAM roles (limit by bucket + key prefix conditions)
- Short pre-signed TTL, validate `etag` on finalize
- Rate limiting and per-user quotas (prevent cost surprises)

---

## Minimal DTOs (shared)

```go
type PresignUploadRequest struct {
  FolderID    string `json:"folder_id"`  // inside a Box
  Name        string `json:"name"`
  Size        int64  `json:"size"`
  ContentType string `json:"content_type"`
}
type PresignUploadResponse struct {
  FileID    string      `json:"file_id"`
  UploadURL string      `json:"upload_url"`
  Headers   http.Header `json:"headers"`
  Method    string      `json:"method"` // "PUT"
}
type CompleteUploadRequest struct {
  Size int64  `json:"size"`
  ETag string `json:"etag"`
}
type PresignDownloadResponse struct {
  DownloadURL string `json:"download_url"`
  Method      string `json:"method"` // "GET"
}
```

---

## CLI Path Parser (notes)

- Accept human paths like `/school/fall-2025:/assignments/lab1/notes.zip`
- Resolve to `{section_id, box_id, folder_id, name}` using `/resolve` API
- Support ID-based flags for scripting: `--section-id`, `--box-id`, `--folder-id`

---

Happy building 🚀
