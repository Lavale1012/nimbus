package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/utils/helpers"
	"github.com/spf13/cobra"
)

type fileListResponse struct {
	Files []FileEntry `json:"files"`
}

var fileListCmd = &cobra.Command{
	Use:   "files",
	Short: "List all files in the current box",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		endpoint := fmt.Sprintf(
			"http://nim.test/v1/api/files?box_name=%s",
			url.QueryEscape(CurrentBox),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return fmt.Errorf("error fetching files: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to list files: %s — %s", resp.Status, string(errBody))
		}

		var result fileListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(result.Files) == 0 {
			fmt.Printf("%s: (no files)\n", CurrentBox)
			return nil
		}

		fmt.Printf("%s — %d file(s)\n\n", CurrentBox, len(result.Files))
		for _, f := range result.Files {
			fmt.Printf("  %-40s %s\n", f.Name, helpers.FormatSize(f.Size))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fileListCmd)
}
