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
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	folderNameFlag string
	folderDestFlag string
)

var folderCmd = &cobra.Command{
	Use:   "cdir <folder-name> [destination]",
	Short: "Create a new folder",
	Long:  "Create a new folder in the current box. If destination is not provided, creates in root.",
	Args:  cobra.RangeArgs(1, 2),

	Example: `nim cdir my-folder
nim cdir my-folder path/to/parent`,
	RunE: func(cmd *cobra.Command, args []string) error {
		folderNameFlag = args[0]
		if len(args) > 1 {
			folderDestFlag = args[1]
		}

		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer RDB.Close()

		IsLoggedIn, err := helpers.SessionExists(RDB)
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
		CurrentPath, err := cache.GetCurrentPath(RDB)
		if err != nil {
			return fmt.Errorf("err: %s", err)
		}
		// endpoint := fmt.Sprintf(
		// 	"http://nim.test/v1/api/folders?box_name=%s&foldername=%s&dest=%s",
		// 	url.QueryEscape(CurrentBox),
		// 	url.QueryEscape(folderNameFlag),
		// 	url.QueryEscape(folderDestFlag),
		// )
		endpoint := fmt.Sprintf(
			"http://nim.test/v1/api/folders?box_name=%s&path=%s&folder_name=%s",
			url.QueryEscape(CurrentBox),
			url.QueryEscape(CurrentPath),
			url.QueryEscape(folderNameFlag),
		)

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}
		if jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		// Create spinner
		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetDescription("Creating folder..."),
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
			return fmt.Errorf("error creating folder: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err == nil {
				if folder, ok := result["folder"].(string); ok {
					fmt.Printf("Folder created: %s\n", folder)
				} else {
					fmt.Println("Folder created successfully")
				}
			} else {
				fmt.Println("Folder created successfully")
			}
		} else {
			var errResp map[string]interface{}
			if err := json.Unmarshal(body, &errResp); err == nil {
				if errMsg, ok := errResp["error"].(string); ok {
					return fmt.Errorf("failed to create folder: %s", errMsg)
				}
			}
			return fmt.Errorf("failed to create folder: %s", resp.Status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(folderCmd)
}
