package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// ResetPasswordRequest is the JSON body sent to /v1/api/auth/users/reset-password.
type ResetPasswordRequest struct {
	Email       string `json:"email"`
	PassKey     string `json:"passkey"`
	NewPassword string `json:"new_password"`
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

		fmt.Println("Forgot your password? Type 'r' at the email prompt to reset it.")
		fmt.Print("\n")

		// Prompt for email and validate format before sending to the server.
		fmt.Print("Enter email (or 'r' to reset password): ")
		fmt.Scanln(&loginRequest.Email)

		// Branch into the password-reset flow.
		if strings.EqualFold(strings.TrimSpace(loginRequest.Email), "r") {
			return runPasswordReset()
		}

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

// runPasswordReset drives the interactive password-reset flow. The user proves
// their identity with the passkey chosen at registration, then sets a new
// password. Secrets (passkey, new password) are read without echoing to the
// terminal. On success the user is directed to log in normally.
func runPasswordReset() error {
	var req ResetPasswordRequest

	fmt.Print("\n--- Password Reset ---\n\n")

	fmt.Print("Enter email: ")
	fmt.Scanln(&req.Email)
	if req.Email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if !helpers.IsEmailValid(req.Email) {
		return fmt.Errorf("invalid email format")
	}

	fmt.Print("Enter passkey (4 characters): ")
	passkey, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	req.PassKey = string(passkey)
	if req.PassKey == "" {
		return fmt.Errorf("passkey cannot be empty")
	}

	fmt.Print("Enter new password: ")
	newPass, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	req.NewPassword = string(newPass)
	if req.NewPassword == "" {
		return fmt.Errorf("password cannot be empty")
	}

	fmt.Print("Confirm new password: ")
	confirm, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	if req.NewPassword != string(confirm) {
		return fmt.Errorf("passwords do not match")
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, config.BaseURL+"/v1/api/auth/users/reset-password", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	stop := animations.Spinner("Resetting password...")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
	stop()
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Surface the server's error message when available.
		var errResp struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(resp.Body).Decode(&errResp); errResp.Error != "" {
			return fmt.Errorf("password reset failed: %s", errResp.Error)
		}
		return fmt.Errorf("password reset failed: %s", resp.Status)
	}

	fmt.Println("Password reset successful. You can now run 'nim login' with your new password.")
	return nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.PersistentFlags().String("login", "", "A help for login")
}
