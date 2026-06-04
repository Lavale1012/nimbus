// Package config resolves the active environment on startup and exposes the
// API base URL and Redis address to every CLI command.
//
// Set NIM_ENV in your shell before running a command:
//
//	NIM_ENV=local nim login          → hits http://localhost:8080
//	NIM_ENV=prod  NIM_API_URL=https://… nim login  → hits the ALB
//
// Defaults to "local" when NIM_ENV is not set.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// BaseURL is the API server root that all CLI commands prepend to their paths.
var BaseURL string

// RedisAddr is the address of the local Redis instance used to cache the
// user's session (JWT token, current box, current path). Redis is always
// local — it's a per-developer session store, not shared infrastructure.
var RedisAddr string

// init runs once when the package is first imported (i.e. at CLI startup).
// It reads NIM_ENV, sets BaseURL and RedisAddr, and exits with a clear error
// if the configuration is invalid so the user knows exactly what to fix.
func init() {
	// Best-effort .env load so developers don't have to export variables manually.
	godotenv.Load()

	env := os.Getenv("NIM_ENV")
	if env == "" {
		env = "local"
	}

	switch env {
	case "local":
		BaseURL = "http://localhost:8080"
		RedisAddr = "localhost:6379"
	case "prod":
		// In prod we don't hardcode the ALB URL because it changes when infra
		// is rebuilt. Set NIM_API_URL to the current ALB DNS name.
		apiURL := os.Getenv("NIM_API_URL")
		if apiURL == "" {
			fmt.Fprintln(os.Stderr, "error: NIM_ENV=prod requires NIM_API_URL to be set")
			os.Exit(1)
		}
		BaseURL = apiURL
		RedisAddr = "localhost:6379"
	default:
		fmt.Fprintf(os.Stderr, "error: unknown NIM_ENV=%q (use \"local\" or \"prod\")\n", env)
		os.Exit(1)
	}
}
