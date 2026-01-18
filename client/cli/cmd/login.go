/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"syscall"
	"time"

	"github.com/nimbus/cli/banner"
	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/utils/helpers"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
	Email   string `json:"email"`
	UserID  uint   `json:"user_id"`
}

func isEmailValid(e string) bool {
	emailRegex := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return emailRegex.MatchString(e)
}

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Implement the login functionality here
		redisClient, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		SessionExist, err := helpers.SessionExists(redisClient)
		if err != nil {
			return fmt.Errorf("failed to check session existence: %w", err)
		}
		if SessionExist {
			fmt.Println("You are already logged in.")
			return nil
		}
		defer redisClient.Close()
		var loginResponse loginResponse
		var loginRequest LoginRequest
		// Populate loginRequest with user input (e.g., flags or prompts)

		banner.ShowLoginBanner()
		fmt.Print("\n")
		fmt.Printf("Enter email: ")
		fmt.Scanln(&loginRequest.Email)
		if loginRequest.Email == "" {
			return fmt.Errorf("email cannot be empty")
		}
		if !isEmailValid(loginRequest.Email) {
			return fmt.Errorf("invalid email format")
		}
		fmt.Printf("Enter password: ")
		password, _ := term.ReadPassword(int(syscall.Stdin))
		loginRequest.Password = string(password)
		fmt.Print("\n")
		if loginRequest.Password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		// For demonstration, print the login request
		MarshaledRequest, _ := json.Marshal(loginRequest)
		endpoint := "http://localhost:8080/v1/api/auth/users/login"
		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(MarshaledRequest))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Create spinner for authentication
		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetDescription("Authenticating..."),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetWidth(15),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionClearOnFinish(),
		)

		// Start spinner in goroutine
		done := make(chan bool)
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					bar.Add(1)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		// Make HTTP request
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)

		// Stop spinner
		done <- true
		bar.Finish()

		if err != nil {
			return fmt.Errorf("failed to perform request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login failed with status: %s", resp.Status)
		}

		if err := json.NewDecoder(resp.Body).Decode(&loginResponse); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Store JWT token and email in Redis cache
		// ctx := context.Background()
		// err = redisClient.HSet(ctx, "user:session", map[string]interface{}{
		// 	"email":     loginRequest.Email,
		// 	"JWT_Token": loginResponse.Token,
		// }).Err()
		err = cache.SetAuthToken(redisClient, loginResponse.UserID, loginResponse.Email, loginResponse.Token)
		if err != nil {
			return fmt.Errorf("failed to cache session: %w", err)
		}
		fmt.Println("Login successful")
		fmt.Printf("Welcome back, %s\n", loginResponse.Email)

		// Perform login logic, e.g., send loginRequest to server API

		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.PersistentFlags().String("login", "", "A help for login")
}
