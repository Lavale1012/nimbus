package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/schollz/progressbar/v3"
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

		IsLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			return fmt.Errorf("you are not logged in, please login first")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}
		if jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		endpoint := fmt.Sprintf(
			"http://nim.test/v1/api/boxes?box_name=%s",
			url.QueryEscape(boxName),
		)

		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetDescription("Creating box..."),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetWidth(15),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionClearOnFinish(),
		)

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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
		if err != nil {
			done <- true
			bar.Finish()
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			done <- true
			bar.Finish()
			return fmt.Errorf("error creating box: %w", err)
		}
		defer resp.Body.Close()

		done <- true
		bar.Finish()

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
