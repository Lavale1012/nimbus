package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(key string) (string, error) {
	env := os.Getenv("NIM_ENV")
	candidates := []string{
		".env." + env,
		".env",
		"../.env." + env,
		"../.env",
		"../../.env." + env,
		"../../.env",
	}
	for _, path := range candidates {
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
