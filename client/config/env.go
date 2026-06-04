package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var BaseURL string
var RedisAddr string

func init() {
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
