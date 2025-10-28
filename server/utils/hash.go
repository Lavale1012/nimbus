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
