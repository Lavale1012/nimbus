package tests

import (
	"testing"

	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
)

// TestPasswordHash_Success tests that password hashing works correctly
func TestPasswordHash_Success(t *testing.T) {
	password := "MySecurePassword123!"

	hash, err := utils.PasswordHash(password)

	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash, "Hash should not equal plain password")
	assert.Greater(t, len(hash), 50, "Bcrypt hash should be at least 50 characters")
}

// TestPasswordHash_EmptyPassword tests hashing an empty password
func TestPasswordHash_EmptyPassword(t *testing.T) {
	password := ""

	hash, err := utils.PasswordHash(password)

	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
}

// TestPasswordHash_DifferentHashesForSamePassword tests that the same password produces different hashes (due to salt)
func TestPasswordHash_DifferentHashesForSamePassword(t *testing.T) {
	password := "SamePassword123!"

	hash1, err1 := utils.PasswordHash(password)
	hash2, err2 := utils.PasswordHash(password)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, hash1, hash2, "Same password should produce different hashes due to salt")
}

// TestCheckPasswordHash_Success tests successful password verification
func TestCheckPasswordHash_Success(t *testing.T) {
	password := "CorrectPassword123!"

	hash, err := utils.PasswordHash(password)
	assert.NoError(t, err)

	result := utils.VerifyPasswordHash(password, hash)
	assert.True(t, result, "Password should match its hash")
}

// TestCheckPasswordHash_WrongPassword tests password verification with wrong password
func TestCheckPasswordHash_WrongPassword(t *testing.T) {
	password := "CorrectPassword123!"
	wrongPassword := "WrongPassword456!"

	hash, err := utils.PasswordHash(password)
	assert.NoError(t, err)

	result := utils.VerifyPasswordHash(wrongPassword, hash)
	assert.False(t, result, "Wrong password should not match hash")
}

// TestCheckPasswordHash_EmptyPassword tests password verification with empty password
func TestCheckPasswordHash_EmptyPassword(t *testing.T) {
	password := "CorrectPassword123!"

	hash, err := utils.PasswordHash(password)
	assert.NoError(t, err)

	result := utils.VerifyPasswordHash("", hash)
	assert.False(t, result, "Empty password should not match hash")
}

// TestCheckPasswordHash_InvalidHash tests password verification with invalid hash
func TestCheckPasswordHash_InvalidHash(t *testing.T) {
	password := "CorrectPassword123!"
	invalidHash := "not-a-valid-bcrypt-hash"

	result := utils.VerifyPasswordHash(password, invalidHash)
	assert.False(t, result, "Invalid hash should return false")
}

// TestCheckPasswordHash_CaseSensitive tests that password checking is case-sensitive
func TestCheckPasswordHash_CaseSensitive(t *testing.T) {
	password := "Password123!"

	hash, err := utils.PasswordHash(password)
	assert.NoError(t, err)

	result := utils.VerifyPasswordHash("password123!", hash)
	assert.False(t, result, "Password check should be case-sensitive")
}

// TestGenerateSecureID_Success tests secure ID generation
func TestGenerateSecureID_Success(t *testing.T) {
	id, err := utils.GenerateSecureID()

	assert.NoError(t, err)
	assert.NotZero(t, id, "Generated ID should not be zero")
}

// TestGenerateSecureID_Uniqueness tests that generated IDs are unique
func TestGenerateSecureID_Uniqueness(t *testing.T) {
	iterations := 1000
	ids := make(map[uint]bool)

	for i := 0; i < iterations; i++ {
		id, err := utils.GenerateSecureID()
		assert.NoError(t, err)

		// Check if ID is unique
		if ids[id] {
			t.Errorf("Duplicate ID generated: %d", id)
		}
		ids[id] = true
	}

	assert.Equal(t, iterations, len(ids), "All generated IDs should be unique")
}

// TestGenerateSecureID_NonSequential tests that IDs are not sequential
func TestGenerateSecureID_NonSequential(t *testing.T) {
	id1, err1 := utils.GenerateSecureID()
	id2, err2 := utils.GenerateSecureID()
	id3, err3 := utils.GenerateSecureID()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)

	// IDs should not be sequential (e.g., id2 != id1 + 1)
	assert.NotEqual(t, id1+1, id2, "IDs should not be sequential")
	assert.NotEqual(t, id2+1, id3, "IDs should not be sequential")
}

// TestGenerateSecureID_Randomness tests that IDs have good distribution
func TestGenerateSecureID_Randomness(t *testing.T) {
	iterations := 100
	var sum uint64

	for i := 0; i < iterations; i++ {
		id, err := utils.GenerateSecureID()
		assert.NoError(t, err)
		sum += uint64(id)
	}

	// Check that average is reasonably high (indicating good use of bit space)
	average := sum / uint64(iterations)
	assert.Greater(t, average, uint64(1000000), "Average ID should be reasonably large, indicating good randomness")
}

// TestGenerateSecureID_PositiveValues tests that all generated IDs are positive
func TestGenerateSecureID_PositiveValues(t *testing.T) {
	iterations := 1000

	for i := 0; i < iterations; i++ {
		id, err := utils.GenerateSecureID()
		assert.NoError(t, err)
		assert.Greater(t, id, uint(0), "Generated ID should be positive")
	}
}

// BenchmarkPasswordHash benchmarks password hashing performance
func BenchmarkPasswordHash(b *testing.B) {
	password := "BenchmarkPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.PasswordHash(password)
	}
}

// BenchmarkCheckPasswordHash benchmarks password verification performance
func BenchmarkCheckPasswordHash(b *testing.B) {
	password := "BenchmarkPassword123!"
	hash, _ := utils.PasswordHash(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.VerifyPasswordHash(password, hash)
	}
}

// BenchmarkGenerateSecureID benchmarks ID generation performance
func BenchmarkGenerateSecureID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.GenerateSecureID()
	}
}
