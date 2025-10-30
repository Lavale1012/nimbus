package utils

import (
	"crypto/rand"
	"encoding/binary"

	"golang.org/x/crypto/bcrypt"
)

func PasswordHash(pw string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pw), 14)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func VerifyPasswordHash(pw, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	return err == nil
}

// GenerateSecureID generates a cryptographically secure random uint ID
// Returns a large random ID suitable for BoxID (up to 63 bits)
func GenerateSecureID() (uint, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	// Convert bytes to uint64, then to uint
	// Use only positive values by masking the sign bit
	id := binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF
	return uint(id), nil
}

// GenerateUserID generates a random 8 digit user ID
// Range: 10,000,000 to 99,999,999
func GenerateUserID() (uint, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	// Convert to uint32 for smaller range
	id := binary.BigEndian.Uint32(b[:])
	// Map to range 10,000,000 to 99,999,999
	// 90,000,000 possible values
	id = (id % 90000000) + 10000000
	return uint(id), nil
}
