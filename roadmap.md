# 🗺️ Nimbus CLI Roadmap

> **Status**: 🚧 Currently in Phase 1 (MVP) development

## Overview

Nimbus CLI is evolving through structured development phases, from a basic MVP with core file operations to a comprehensive enterprise-ready cloud file management platform.

---

## 🎯 Phase 1: MVP (Local Development)

**Goal**: Establish core file operations with local development environment

### ✅ Completed Features

- ✅ **Project Structure & Architecture**
  - Go-based CLI with Cobra framework
  - REST API server with Gin
  - Modular codebase with client/server separation
  - Docker Compose for local development

- ✅ **Core File Operations**
  - File upload (`nim post --file <path>`)
  - File download (`nim get --file <key> --output <path>`)
  - File deletion (`nim del --file <filename>`)

- ✅ **S3 Integration**
  - AWS S3 client configuration
  - LocalStack for local S3 emulation
  - Direct S3 upload/download operations
  - Consolidated S3 operations handler

- ✅ **CLI Experience**
  - Command-line argument parsing
  - Progress indicators (upload progress bar, delete spinner)
  - Error handling and user feedback
  - Consistent emoji-based success/error messages

- ✅ **Development Environment**
  - Environment variable configuration
  - Local PostgreSQL setup
  - S3 endpoint configuration for LocalStack
  - Build and test automation

- ✅ **Documentation**
  - Comprehensive README with setup instructions
  - API endpoint documentation
  - Development guidelines
  - Visual enhancements with emojis and better formatting

### 🔄 In Progress

- **HTTP Route Fixes**: Ensuring all endpoints work correctly with proper parameter handling
- **Error Response Handling**: Improving server error message propagation to CLI

### 📋 Remaining MVP Tasks

- **File Listing**: `nim ls` command to list uploaded files
- **File Metadata**: Show file size, upload date, and other metadata
- **Configuration Management**: CLI config file for default endpoints
- **Comprehensive Testing**: Unit and integration tests for all commands
- **Build Optimization**: Cross-platform build scripts and releases

---

## 🔐 Phase 2: Authentication & Authorization

**Goal**: Secure the platform with user management and access control

### 🎯 Planned Features

- **User Authentication**
  - OIDC integration (Auth0/Cognito/Clerk)
  - JWT token management
  - `nim login` and `nim logout` commands
  - `nim whoami` command for current user info

- **Authorization Framework**
  - Role-based access control (RBAC)
  - Resource-level permissions
  - API key management for programmatic access

- **Security Enhancements**
  - Pre-signed S3 URLs for secure uploads/downloads
  - Audit logging for all operations
  - Encryption in transit and at rest

---

## 📦 Phase 3: Hierarchical Organization

**Goal**: Implement the full Box → Folder → File hierarchy

### 🎯 Planned Features

- **Box Management**
  - `nim box create <name>` - Create new box
  - `nim box ls` - List user's boxes
  - `nim box rm <name>` - Delete box

- **Folder Operations**
  - `nim mkdir <box>:/<path>` - Create folders
  - `nim ls <box>:/[path]` - List box/folder contents
  - `nim rmdir <box>:/<path>` - Remove folders

- **Enhanced File Operations**
  - `nim upload <file> <box>:/<path>` - Upload to specific location
  - `nim download <box>:/<path> [--output <path>]` - Download from hierarchy
  - `nim rm <box>:/<path>` - Remove files from hierarchy
  - `nim mv <box>:/<src> <box>:/<dest>` - Move/rename files

- **Path Resolution**
  - Hierarchical path format: `BoxName:/folder/subfolder/file.ext`
  - Path validation and normalization
  - Symbolic link support

---

## 🤝 Phase 4: Sharing & Collaboration

**Goal**: Enable secure file sharing and team collaboration

### 🎯 Planned Features

- **File Sharing**
  - `nim share <box>:/<path> --user <email>` - Share with specific users
  - `nim share <box>:/<path> --public` - Create public links
  - Time-limited access links
  - Permission levels (read, write, admin)

- **Team Management**
  - Organization/team creation
  - Team member management
  - Shared boxes and folders
  - Team-level permissions

- **Collaboration Features**
  - File comments and annotations
  - Activity feeds and notifications
  - Real-time collaboration indicators

---

## ⚡ Phase 5: Advanced Features

**Goal**: Power-user features and productivity enhancements

### 🎯 Planned Features

- **Search & Discovery**
  - `nim search <query>` - Full-text search across files
  - Metadata-based filtering
  - Tag support and management
  - Recently accessed files

- **Version Control**
  - File versioning and history
  - `nim history <box>:/<path>` - View file versions
  - Version restoration and comparison
  - Automatic backup policies

- **Bulk Operations**
  - `nim sync <local-dir> <box>:/` - Directory synchronization
  - Batch upload/download operations
  - Parallel processing for large operations
  - Resume interrupted transfers

- **Advanced CLI Features**
  - Shell completion (bash, zsh, fish)
  - Configuration profiles for different environments
  - Plugin system for extensibility
  - Backup and restore utilities

---

## 🏢 Phase 6: Enterprise & Compliance

**Goal**: Enterprise-ready features for organizational deployment

### 🎯 Planned Features

- **Enterprise Admin**
  - Admin dashboard and management tools
  - Usage analytics and reporting
  - Quota management and billing
  - Multi-tenant support

- **Compliance & Security**
  - SOC 2, GDPR, HIPAA compliance
  - Data loss prevention (DLP)
  - Advanced audit logging
  - Retention policies and legal holds

- **Integration & Automation**
  - API webhooks and events
  - Third-party integrations (Slack, Teams, etc.)
  - Workflow automation
  - Single sign-on (SSO) enterprise integration

- **Performance & Scale**
  - CDN integration for global access
  - Advanced caching strategies
  - Database sharding and optimization
  - High availability and disaster recovery

---

## 📊 Current Status Summary

### ✅ Completed (Phase 1)
- [x] Project architecture and setup
- [x] Basic file upload/download/delete
- [x] S3 integration with LocalStack
- [x] CLI framework with progress indicators
- [x] Development environment
- [x] Documentation and README

### 🔄 In Progress (Phase 1)
- [ ] Route fixes and error handling
- [ ] File listing functionality
- [ ] Configuration management

### 📅 Next Up (Phase 1)
- [ ] Comprehensive testing
- [ ] Build optimization
- [ ] File metadata display

### 🎯 Success Metrics

**Phase 1 Completion Criteria:**
- ✅ All basic file operations working
- ✅ Local development environment functional
- ✅ CLI user experience polished
- ⏳ Test coverage > 80%
- ⏳ Cross-platform builds available

---

## 🚀 Getting Started

To contribute to the current phase:

1. **Set up development environment** (✅ Complete)
2. **Run existing tests** (⏳ In Progress)
3. **Pick a task from Phase 1 remaining items**
4. **Follow development guidelines in README**

---

## 📝 Notes

- Each phase builds upon the previous one
- MVP prioritizes core functionality over advanced features
- User feedback will influence priority of future phases
- Security and performance are considered throughout all phases

**Last Updated**: December 2024
**Current Phase**: Phase 1 (MVP) - 85% Complete