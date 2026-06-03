# Nimbus Server Test Suite

Comprehensive tests for the Nimbus API server. All tests use an in-memory SQLite database — no external services required.

## Running Tests

```bash
# All tests
cd server && go test ./tests/ -v

# With coverage
cd server && go test ./tests/ -cover

# Single file
cd server && go test ./tests/ -run TestFileList -v
```

## Test Files

### `hash_utils_test.go`

Password hashing and secure ID generation.

| Test | Status |
| --- | --- |
| TestPasswordHash_Success | PASS |
| TestPasswordHash_EmptyPassword | PASS |
| TestPasswordHash_DifferentHashesForSamePassword | PASS |
| TestCheckPasswordHash_Success | PASS |
| TestCheckPasswordHash_WrongPassword | PASS |
| TestCheckPasswordHash_EmptyPassword | PASS |
| TestCheckPasswordHash_InvalidHash | PASS |
| TestCheckPasswordHash_CaseSensitive | PASS |
| TestGenerateSecureID_Success | PASS |
| TestGenerateSecureID_Uniqueness | PASS |
| TestGenerateSecureID_NonSequential | PASS |
| TestGenerateSecureID_Randomness | PASS |
| TestGenerateSecureID_PositiveValues | PASS |

13/13 passing

---

### `userauth_test.go`

User registration validation.

Covers: email format, password strength (length, uppercase, lowercase, digit, special char), duplicate detection, home box auto-creation.

---

### `user_login_test.go`

Login handler (`POST /v1/api/auth/login`).

Covers: JWT returned on success, wrong password rejected, unknown email rejected, missing fields, invalid email format, boxes returned in response, case-sensitive email matching.

---

### `file_handler_test.go`

File handlers: `List`, `Rename`, `Move`.

18 tests covering: unauthorized requests, missing query params, wrong box ownership, file not found, success paths, user isolation (user A cannot see/touch user B's files).

---

### `box_handler_test.go`

Box handlers: `ListBoxes`, `VerifyBoxExist`.

12 tests covering: unauthorized requests, empty box list, multiple boxes returned, box not found, ownership check, duplicate name detection (handler-level SELECT-before-INSERT).

---

### `folder_delete_test.go`

Folder delete handler.

Covers: unauthorized, folder not found, wrong ownership, success with DB cleanup.

---

### `folder_rename_test.go`

Folder rename handler.

Covers: unauthorized, missing params, folder not found, wrong ownership, success.

---

### `file_operations_test.go`

File model CRUD operations and DB associations.

Covers: create/read/delete file records, S3 key queries, cascade delete behavior.

---

## Dependencies

```bash
go get github.com/stretchr/testify/assert
go get gorm.io/driver/sqlite
```

Both are already in `server/go.mod`.
