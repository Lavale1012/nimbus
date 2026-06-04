// Package utils provides shared utility functions for the Nimbus API server.
package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// GetEnv reads an environment variable by key. Before looking up the variable
// it tries to load a .env file so the server works out of the box without
// requiring the caller to export variables manually.
//
// File resolution order (first successful load wins):
//  1. .env.<NIM_ENV>   — environment-specific file in the current directory
//  2. .env             — generic file in the current directory
//  3. ../.env.<NIM_ENV> / ../.env — same two, one directory up
//  4. ../../.env.<NIM_ENV> / ../../.env — two directories up (covers running from a sub-package)
//
// This means: NIM_ENV=local loads .env.local; NIM_ENV=prod loads .env.prod;
// no NIM_ENV falls back to plain .env.
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
