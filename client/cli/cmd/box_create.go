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
	"github.com/spf13/cobra"
)

var createBoxCmd = &cobra.Command{
	Use:     "mkbox <box-name>",
	Short:   "Create a new box",
	Long:    "Create a new box in your Nimbus account.",
	Args:    cobra.ExactArgs(1),
	Example: `nim mkbox my-box`,
	RunE: func(cmd *cobra.Command, args []string) error {
		boxName := args[0]

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
		if err != nil || jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		endpoint := fmt.Sprintf(
			"http://nim.test/v1/api/boxes?box_name=%s",
			url.QueryEscape(boxName),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Creating box...")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("error creating box: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to create box: %s — %s", resp.Status, string(errBody))
		}

		fmt.Printf("Box \"%s\" created successfully\n", boxName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createBoxCmd)
}
