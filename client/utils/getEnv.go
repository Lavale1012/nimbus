// Package utils provides small utility functions for the Nimbus CLI client.
package utils

import (
	"os"

	"github.com/joho/godotenv"
)

// GetEnv reads an environment variable by key. It first tries to load a .env
// file from a few well-known relative paths so the CLI works without requiring
// the developer to export every variable manually.
func GetEnv(key string) (string, error) {
	possiblePaths := []string{
		".env",       // same directory as the executable
		"../.env",    // one level up
		"../../.env", // two levels up (when running from a sub-package during development)
	}

	var err error
	for _, path := range possiblePaths {
		err = godotenv.Load(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", err
	}

	return os.Getenv(key), nil
}
