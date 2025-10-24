# Nimbus CLI Test Suite

This directory contains comprehensive tests for the Nimbus CLI server components.

## Test Files

### 1. `hash_utils_test.go`
Tests for password hashing and secure ID generation utilities.

**Coverage:**
- Password hashing with bcrypt
- Password verification
- Secure ID generation using crypto/rand
- Uniqueness and randomness validation

**Run:** `go test ./tests/hash_utils_test.go -v`

**Status:** ✅ All tests passing (13/13)

### 2. `userauth_test.go`
Tests for user authentication and registration.

**Coverage:**
- User registration validation
- Email format validation
- Password strength requirements
- Duplicate user detection
- Home box creation

**Note:** Currently has model relationship issues that need to be resolved. The FolderModel schema needs to be updated to use proper uint foreign keys instead of strings.

### 3. `file_operations_test.go`
Tests for file operations and database models.

**Coverage:**
- File model CRUD operations
- User and box associations
- S3 key queries
- Cascade delete behavior

**Note:** Requires model schema fixes to run properly.

## Running Tests

### Run all tests:
```bash
cd /Users/lavalebutterfield/Desktop/nim-cli/server
go test ./tests/ -v
```

### Run specific test file:
```bash
go test ./tests/hash_utils_test.go -v
```

### Run with coverage:
```bash
go test ./tests/hash_utils_test.go -v -cover
```

### Run benchmarks:
```bash
go test ./tests/hash_utils_test.go -bench=.
```

## Test Results Summary

### Hash Utils Tests
| Test | Status | Description |
|------|--------|-------------|
| TestPasswordHash_Success | ✅ PASS | Basic password hashing works |
| TestPasswordHash_EmptyPassword | ✅ PASS | Handles empty passwords |
| TestPasswordHash_DifferentHashesForSamePassword | ✅ PASS | Salting produces unique hashes |
| TestCheckPasswordHash_Success | ✅ PASS | Correct password verification |
| TestCheckPasswordHash_WrongPassword | ✅ PASS | Rejects wrong passwords |
| TestCheckPasswordHash_EmptyPassword | ✅ PASS | Rejects empty password attempts |
| TestCheckPasswordHash_InvalidHash | ✅ PASS | Handles invalid hash format |
| TestCheckPasswordHash_CaseSensitive | ✅ PASS | Password checking is case-sensitive |
| TestGenerateSecureID_Success | ✅ PASS | ID generation works |
| TestGenerateSecureID_Uniqueness | ✅ PASS | 1000 IDs all unique |
| TestGenerateSecureID_NonSequential | ✅ PASS | IDs are not sequential |
| TestGenerateSecureID_Randomness | ✅ PASS | Good distribution of values |
| TestGenerateSecureID_PositiveValues | ✅ PASS | All IDs are positive |

**Total: 13/13 passing**

## Known Issues

### Model Relationship Error
The `FolderModel` schema uses string types for `UserID` and `Box` fields, but `BoxModel` tries to establish foreign key relationships expecting uint types. This causes GORM migration errors.

**Fix Required:**
Update `FolderModel` in `/server/models/FolderModel.go`:
```go
type FolderModel struct {
    gorm.Model
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"not null" json:"name"`
    UserID    uint      `gorm:"not null;index" json:"user_id"`  // Change from string to uint
    BoxID     uint      `gorm:"not null;index" json:"box_id"`   // Add BoxID field as uint
    ParentID  *uint     `json:"parent_id"`
    // ... other fields
}
```

## Dependencies

The test suite requires:
- `github.com/stretchr/testify/assert` - Assertion library
- `gorm.io/driver/sqlite` - In-memory database for testing
- `github.com/gin-gonic/gin` - HTTP framework (test mode)

Install dependencies:
```bash
go get github.com/stretchr/testify/assert
go get gorm.io/driver/sqlite
```

## Best Practices

1. **Isolation**: Each test uses an in-memory SQLite database for isolation
2. **Setup/Teardown**: Helper functions (`setupTestDB`, `createTestUser`) manage test state
3. **Table-driven tests**: Multiple test cases with descriptive names
4. **Coverage**: Tests cover success cases, edge cases, and error conditions
5. **Benchmarks**: Performance benchmarks included for critical operations

## Future Improvements

- [ ] Fix model relationship issues
- [ ] Add integration tests with real PostgreSQL
- [ ] Add S3 mocking for file operation tests
- [ ] Add tests for UserLogin functionality
- [ ] Add rate limiting tests
- [ ] Add concurrent operation tests
- [ ] Increase test coverage to 80%+
