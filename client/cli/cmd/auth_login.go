package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"syscall"
	"time"

	"github.com/nimbus/cli/banner"
	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/animations"
	"github.com/nimbus/cli/config"
	"github.com/nimbus/cli/utils/helpers"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// LoginRequest is the JSON body sent to /v1/api/auth/users/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is the JSON body received from /v1/api/auth/login on success.
type LoginResponse struct {
	Message string           `json:"message"`
	Token   string           `json:"token"`
	Email   string           `json:"email"`
	UserID  uint             `json:"user_id"`
	Box     []map[string]any `json:"box"`
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your Nimbus account",
	RunE: func(cmd *cobra.Command, args []string) error {
		var loginResponse LoginResponse
		var loginRequest LoginRequest

		// Connect to the local Redis session cache.
		redisClient, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer redisClient.Close()

		// If a session already exists the user is already logged in — no-op.
		sessionExists, err := cache.SessionExists(redisClient)
		if err != nil {
			return fmt.Errorf("failed to check session existence: %w", err)
		}
		if sessionExists {
			fmt.Println("You are already logged in.")
			return nil
		}

		banner.ShowLoginBanner()
		fmt.Print("\n")

		// Prompt for email and validate format before sending to the server.
		fmt.Print("Enter email: ")
		fmt.Scanln(&loginRequest.Email)
		if loginRequest.Email == "" {
			return fmt.Errorf("email cannot be empty")
		}
		if !helpers.IsEmailValid(loginRequest.Email) {
			return fmt.Errorf("invalid email format")
		}

		// term.ReadPassword reads the password without echoing it to the terminal.
		fmt.Print("Enter password: ")
		password, _ := term.ReadPassword(int(syscall.Stdin))
		loginRequest.Password = string(password)
		fmt.Print("\n")
		if loginRequest.Password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		body, _ := json.Marshal(loginRequest)
		req, err := http.NewRequest(http.MethodPost, config.BaseURL+"/v1/api/auth/users/login", bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		stop := animations.Spinner("Authenticating...")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("failed to perform request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login failed: %s", resp.Status)
		}

		if err := json.NewDecoder(resp.Body).Decode(&loginResponse); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Cache the session locally so subsequent commands don't need to re-authenticate.
		if err := cache.SetAuthToken(redisClient, loginResponse.UserID, loginResponse.Email, loginResponse.Box, loginResponse.Token); err != nil {
			return fmt.Errorf("failed to cache session: %w", err)
		}
		if err := cache.StoreBoxes(redisClient, loginResponse.Box); err != nil {
			return fmt.Errorf("failed to store boxes: %w", err)
		}

		fmt.Printf("Login successful\nWelcome back, %s\n", loginResponse.Email)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.PersistentFlags().String("login", "", "A help for login")
}
