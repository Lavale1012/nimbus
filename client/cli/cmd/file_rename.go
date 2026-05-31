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
	"github.com/spf13/cobra"
)

var fileRenameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename a file",
	Example: `nim rename --key users/nim-user-1/boxes/Home-Box/notes.txt --name new_notes.txt`,
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

		s3Key, _ := cmd.Flags().GetString("key")
		newName, _ := cmd.Flags().GetString("name")
		if s3Key == "" || newName == "" {
			return fmt.Errorf("--key and --name are required")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		endpoint := fmt.Sprintf(
			"http://nim.test/v1/api/files/rename?box_name=%s&key=%s&new_name=%s",
			url.QueryEscape(CurrentBox),
			url.QueryEscape(s3Key),
			url.QueryEscape(newName),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return fmt.Errorf("error renaming file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("rename failed: %s — %s", resp.Status, string(errBody))
		}

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Printf("Renamed to %s\n", newName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fileRenameCmd)
	fileRenameCmd.Flags().String("key", "", "S3 key of the file to rename (required)")
	fileRenameCmd.Flags().String("name", "", "New name for the file (required)")
	fileRenameCmd.MarkFlagRequired("key")
	fileRenameCmd.MarkFlagRequired("name")
}
