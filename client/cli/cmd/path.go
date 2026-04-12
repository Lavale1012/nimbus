package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/types"
	"github.com/nimbus/cli/utils/helpers"
	"github.com/spf13/cobra"
)

// cdCmd represents the cd command - change current path
var cdCmd = &cobra.Command{
	Use:   "cd [path]",
	Short: "Change current directory path",
	Long:  `Change the current working directory within your box. Use relative or absolute paths.`,
	Args:  cobra.MaximumNArgs(1),
	Example: `nim cd              # Go to box root
nim cd myfolder     # Go to myfolder (relative)
nim cd /some/path   # Go to absolute path
nim cd ..           # Go up one level`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Connect to Redis
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}
		// 2. Check if logged in
		IsLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			fmt.Println("You are not logged in.")
			return nil
		}
		// 3. Check if box is set
		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		// 4. Get current path from cache (empty string means root)
		currentPath, _ := cache.GetCurrentPath(RDB)

		// 5. Calculate new path (handle .., relative, absolute)
		var newPath string
		if len(args) == 0 {
			// nim cd → go to root
			newPath = ""
		} else {
			targetPath := args[0]
			if strings.HasPrefix(targetPath, "/") {
				// Absolute path - use as-is (strip leading slash for storage)
				newPath = strings.TrimPrefix(targetPath, "/")
			} else if targetPath == ".." {
				// Go up one level
				if currentPath == "" {
					// Already at root, stay at root
					newPath = ""
				} else {
					// Remove last path segment
					lastSlash := strings.LastIndex(currentPath, "/")
					if lastSlash == -1 {
						newPath = ""
					} else {
						newPath = currentPath[:lastSlash]
					}
				}
			} else if strings.Contains(targetPath, "..") {
				// Handle paths with .. in them (e.g., ../sibling, foo/../bar)
				combined := path.Join(currentPath, targetPath)
				newPath = path.Clean(combined)
				// path.Clean may return "." for empty, convert to ""
				if newPath == "." {
					newPath = ""
				}
				// Prevent escaping root (path.Clean handles this but double-check)
				if strings.HasPrefix(newPath, "..") {
					newPath = ""
				}
			} else {
				// Relative path - append to current
				if currentPath == "" {
					newPath = targetPath
				} else {
					newPath = currentPath + "/" + targetPath
				}
			}
		}

		// Clean up any double slashes or trailing slashes
		newPath = strings.Trim(newPath, "/")
		for strings.Contains(newPath, "//") {
			newPath = strings.ReplaceAll(newPath, "//", "/")
		}

		// 6. Validate path exists (optional - query server)
		// TODO: Could add server validation here

		// 7. Save new path to cache
		if err := cache.SetCurrentPath(RDB, newPath); err != nil {
			return fmt.Errorf("failed to save path: %w", err)
		}

		// Display the new path
		if newPath == "" {
			fmt.Printf("%s/\n", CurrentBox)
		} else {
			fmt.Printf("%s/%s\n", CurrentBox, newPath)
		}

		return nil
	},
}

// pwdCmd represents the pwd command - print working directory
var pwdCmd = &cobra.Command{
	Use:   "pwd",
	Short: "Print current directory path",
	Long:  `Display the current working directory path within your box.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Connect to Redis

		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}
		// 2. Check if logged in
		IsLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			fmt.Println("You are not logged in.")
			return nil
		}
		// 3. Check if box is set
		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		// 4. Get current path from cache
		CurrentPath, _ := cache.GetCurrentPath(RDB)

		// 5. Print: box_name:/current/path
		if CurrentPath == "" {
			fmt.Printf("%s/\n", CurrentBox)
		} else {
			fmt.Printf("%s/%s\n", CurrentBox, CurrentPath)
		}

		return nil
	},
}

// lsCmd represents the ls command - list directory contents
var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List contents of current or specified directory",
	Long:  `List files and folders in the current directory or a specified path.`,
	Args:  cobra.MaximumNArgs(1),
	Example: `nim ls              # List current directory
nim ls myfolder     # List contents of myfolder
nim ls /some/path   # List contents of absolute path`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Connect to Redis
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}
		defer RDB.Close()

		// 2. Check if logged in
		IsLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			return fmt.Errorf("you are not logged in, please login first")
		}

		// 3. Check if box is set
		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		// 4. Get current path from cache
		currentPath, _ := cache.GetCurrentPath(RDB)

		// 5. Determine target path (current or arg)
		targetPath := currentPath
		if len(args) > 0 {
			arg := args[0]
			if strings.HasPrefix(arg, "/") {
				targetPath = strings.TrimPrefix(arg, "/")
			} else {
				if currentPath == "" {
					targetPath = arg
				} else {
					targetPath = currentPath + "/" + arg
				}
			}
		}
		targetPath = strings.Trim(targetPath, "/")

		// 6. Query server for directory listing
		endpoint := fmt.Sprintf(
			"http://nim.test/v1/api/folders?box_name=%s&path=%s",
			url.QueryEscape(CurrentBox),
			url.QueryEscape(targetPath),
		)

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error fetching directory listing: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errBody map[string]string
			json.NewDecoder(resp.Body).Decode(&errBody)
			if msg, ok := errBody["error"]; ok {
				return fmt.Errorf("%s", msg)
			}
			return fmt.Errorf("failed to list directory: %s", resp.Status)
		}

		var listing types.ListResponse
		if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// 7. Display results
		displayPath := CurrentBox + listing.Path
		fmt.Printf("%s\n\n", displayPath)

		if len(listing.Folders) == 0 && len(listing.Files) == 0 {
			fmt.Println("  (empty)")
			return nil
		}

		for _, f := range listing.Folders {
			fmt.Printf("  [dir]  %s/\n", f.Name)
		}
		for _, f := range listing.Files {
			fmt.Printf("  [file] %-30s %s\n", f.Name, helpers.FormatSize(f.Size))
		}

		fmt.Printf("\n  %d folder(s), %d file(s)\n", len(listing.Folders), len(listing.Files))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
	rootCmd.AddCommand(pwdCmd)
	rootCmd.AddCommand(lsCmd)
}
