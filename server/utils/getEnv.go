package utils

import (
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(key string) (string, error) {
	err := godotenv.Load("../../.env")
	if err != nil {
		return "", err
	}
	return os.Getenv(key), nil
}
