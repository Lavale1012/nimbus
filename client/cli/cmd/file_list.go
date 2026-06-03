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

type lsResponse struct {
	FolderName string        `json:"folder_name"`
	Path       string        `json:"path"`
	Files      []FileEntry   `json:"files"`
	Folders    []FolderEntry `json:"folders"`
}

var listPathFlag string

var fileListCmd = &cobra.Command{
	Use:     "ls",
	Short:   "List files and folders in the current box or a specific folder",
	Example: "nim ls\nnim ls --path documents\nnim ls --path documents/projects",
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

		currentBox, err := cache.GetBoxName(RDB)
		if err != nil || currentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		endpoint := fmt.Sprintf(
			"http://localhost:8080/v1/api/folders?box_name=%s",
			url.QueryEscape(currentBox),
		)
		if listPathFlag != "" {
			endpoint += "&path=" + url.QueryEscape(listPathFlag)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return fmt.Errorf("error fetching contents: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to list contents: %s — %s", resp.Status, string(errBody))
		}

		var result lsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		totalItems := len(result.Folders) + len(result.Files)
		if totalItems == 0 {
			fmt.Printf("%s%s — (empty)\n", currentBox, result.Path)
			return nil
		}

		fmt.Printf("%s%s — %d item(s)\n\n", currentBox, result.Path, totalItems)

		for _, f := range result.Folders {
			fmt.Printf("  [dir]  %s/\n", f.Name)
		}
		for _, f := range result.Files {
			fmt.Printf("  [file] %-38s %s\n", f.Name, helpers.FormatSize(f.Size))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fileListCmd)
	fileListCmd.Flags().StringVarP(&listPathFlag, "path", "p", "", "Folder path to list (e.g. documents or documents/projects)")
}
