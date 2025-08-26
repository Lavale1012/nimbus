package utils

import (
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(key string) (string, error) {
	// Try different possible locations for .env file
	possiblePaths := []string{
		".env",       // same directory as executable
		"../.env",    // parent directory
		"../../.env", // grandparent directory (from utils/)
	}

	var err error
	for _, path := range possiblePaths {
		err = godotenv.Load(path)
		if err == nil {
			break // Successfully loaded
		}
	}

	// If all paths failed, return the last error
	if err != nil {
		return "", err
	}

	return os.Getenv(key), nil
}
