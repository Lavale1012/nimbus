package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(key string) (string, error) {
	// Try to load a .env file (optional — env vars may already be set by the OS/container)
	possiblePaths := []string{
		".env",
		"../.env",
		"../../.env",
	}
	for _, path := range possiblePaths {
		if godotenv.Load(path) == nil {
			break
		}
	}

	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("environment variable %q is not set", key)
	}
	return val, nil
}
