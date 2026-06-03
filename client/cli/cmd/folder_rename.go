package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/animations"
	"github.com/spf13/cobra"
)

var renameFolderCmd = &cobra.Command{
	Use:     "mvdir <folder-name> <new-name>",
	Short:   "Rename a folder",
	Args:    cobra.ExactArgs(2),
	Example: `nim mvdir old-name new-name`,
	RunE: func(cmd *cobra.Command, args []string) error {
		folderName := args[0]
		newName := args[1]

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

		currentBox, err := cache.GetBoxName(RDB)
		if err != nil || currentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		currentPath, _ := cache.GetCurrentPath(RDB)

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil || jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		endpoint := fmt.Sprintf(
			"http://localhost:8080/v1/api/folders/rename?box_name=%s&path=%s&folder_name=%s&new_name=%s",
			url.QueryEscape(currentBox),
			url.QueryEscape(currentPath),
			url.QueryEscape(folderName),
			url.QueryEscape(newName),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Renaming folder...")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("error renaming folder: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			fmt.Printf("Folder '%s' renamed to '%s'\n", folderName, newName)
		case http.StatusNotFound:
			return fmt.Errorf("folder '%s' not found", folderName)
		case http.StatusConflict:
			return fmt.Errorf("a folder named '%s' already exists", newName)
		default:
			return fmt.Errorf("failed to rename folder: %s", resp.Status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameFolderCmd)
}
