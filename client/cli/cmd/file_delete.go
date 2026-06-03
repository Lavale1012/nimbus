package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/animations"
	"github.com/spf13/cobra"
)

var deleteFilePathFlag string

var deleteFileCmd = &cobra.Command{
	Use:   "del",
	Short: "Delete a file",
	RunE: func(cmd *cobra.Command, args []string) error {
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer RDB.Close()

		isLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !isLoggedIn {
			return fmt.Errorf("you are not logged in, please login first")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		endpoint := "http://localhost:8080/v1/api/files/" + deleteFilePathFlag

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Deleting " + deleteFilePathFlag + "...")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("error deleting file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to delete file: %s — %s", resp.Status, string(errBody))
		}

		fmt.Printf("Deleted %s\n", deleteFilePathFlag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteFileCmd)
	deleteFileCmd.Flags().StringVarP(&deleteFilePathFlag, "file", "f", "", "S3 key of the file to delete (required)")
	deleteFileCmd.MarkFlagRequired("file")
}
