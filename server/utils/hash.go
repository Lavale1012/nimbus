package utils

import (
	"crypto/rand"
	"encoding/binary"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHash hashes a plain-text password using bcrypt with cost 14.
// Cost 14 is deliberately slow — it makes brute-force attacks expensive even
// if an attacker obtains the hash. Always store the result, never the
// original password.
func PasswordHash(pw string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pw), 14)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// VerifyPasswordHash compares a plain-text password against a bcrypt hash.
// Returns true only if the password produced the hash. bcrypt's timing is
// constant regardless of where the strings differ, which prevents timing attacks.
func VerifyPasswordHash(pw, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	return err == nil
}

// GenerateSecureID generates a cryptographically random uint suitable for use
// as a BoxID. The sign bit is masked so the value always fits in a signed
// 63-bit integer (safe for PostgreSQL BIGINT columns).
func GenerateSecureID() (uint, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	id := binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF
	return uint(id), nil
}

// GenerateUserID generates a random 8-digit user ID in the range
// 10,000,000 – 99,999,999. Using a fixed-width ID makes them predictable in
// length while still being hard to guess.
func GenerateUserID() (uint, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	id := binary.BigEndian.Uint32(b[:])
	// Modulo maps the full uint32 range onto 90,000,000 possible values,
	// then we shift up so the result starts at 10,000,000.
	id = (id % 90000000) + 10000000
	return uint(id), nil
}
