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
	"github.com/nimbus/cli/cli/animations"
	"github.com/nimbus/cli/config"
	"github.com/spf13/cobra"
)

var folderNameFlag string

var folderCmd = &cobra.Command{
	Use:   "cdir <folder-name> [destination]",
	Short: "Create a new folder",
	Long:  "Create a new folder in the current box. If destination is not provided, creates in root.",
	Args:  cobra.RangeArgs(1, 2),
	Example: `nim cdir my-folder
nim cdir my-folder path/to/parent`,
	RunE: func(cmd *cobra.Command, args []string) error {
		folderNameFlag = args[0]
		// NOTE: an optional destination path (args[1]) is accepted by the command
		// signature but not yet wired into folder creation.

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

		currentBox, err := cache.GetBoxName(RDB)
		if err != nil || currentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		// Use the cached current path as the parent for the new folder, unless
		// an explicit destination was given as a second argument.
		currentPath, _ := cache.GetCurrentPath(RDB)

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil || jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		endpoint := fmt.Sprintf(
			config.BaseURL+"/v1/api/folders?box_name=%s&path=%s&folder_name=%s",
			url.QueryEscape(currentBox),
			url.QueryEscape(currentPath),
			url.QueryEscape(folderNameFlag),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Creating folder...")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("error creating folder: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var errResp map[string]interface{}
			if json.Unmarshal(body, &errResp) == nil {
				if msg, ok := errResp["error"].(string); ok {
					return fmt.Errorf("failed to create folder: %s", msg)
				}
			}
			return fmt.Errorf("failed to create folder: %s", resp.Status)
		}

		var result map[string]interface{}
		if json.Unmarshal(body, &result) == nil {
			if folder, ok := result["folder"].(string); ok {
				fmt.Printf("Folder created: %s\n", folder)
				return nil
			}
		}
		fmt.Println("Folder created successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(folderCmd)
}
