package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/animations"
	"github.com/nimbus/cli/config"
	"github.com/spf13/cobra"
)

var deleteBoxNameFlag string

var deleteBoxCmd = &cobra.Command{
	Use:     "rmbox <box-name>",
	Short:   "Delete a box and all its contents",
	Long:    "Permanently delete a box and everything inside it. This cannot be undone.",
	Args:    cobra.ExactArgs(1),
	Example: `nim rmbox my-box`,
	RunE: func(cmd *cobra.Command, args []string) error {
		deleteBoxNameFlag = args[0]

		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer func() { _ = RDB.Close() }()

		isLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !isLoggedIn {
			return fmt.Errorf("you are not logged in, please login first")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil || jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		endpoint := fmt.Sprintf(
			config.BaseURL+"/v1/api/boxes?box_name=%s",
			url.QueryEscape(deleteBoxNameFlag),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Deleting box...")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("error deleting box: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to delete box: %s — %s", resp.Status, string(errBody))
		}

		fmt.Printf("Box \"%s\" deleted successfully\n", deleteBoxNameFlag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteBoxCmd)
}
