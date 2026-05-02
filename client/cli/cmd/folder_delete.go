package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var deleteFolderCmd = &cobra.Command{
	Use:     "rmdir <folder-name>",
	Short:   "Delete a folder and all its contents",
	Args:    cobra.ExactArgs(1),
	Example: `nim rmdir my-folder`,
	RunE: func(cmd *cobra.Command, args []string) error {
		folderName := args[0]

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
			"http://nim.test/v1/api/folders?box_name=%s&path=%s&folder_name=%s",
			url.QueryEscape(currentBox),
			url.QueryEscape(currentPath),
			url.QueryEscape(folderName),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetDescription("Deleting folder..."),
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

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		done <- true
		bar.Finish()

		if err != nil {
			return fmt.Errorf("error deleting folder: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("folder '%s' not found", folderName)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("failed to delete folder: %s", resp.Status)
		}

		fmt.Printf("Folder '%s' deleted successfully\n", folderName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteFolderCmd)
}
