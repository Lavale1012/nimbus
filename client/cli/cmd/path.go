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
	"github.com/nimbus/cli/config"
	"github.com/nimbus/cli/utils/helpers"
	"github.com/spf13/cobra"
)

// ListResponse is the JSON returned by GET /v1/api/folders.
// It describes the contents (files + sub-folders) of a single directory.
type ListResponse struct {
	FolderID   *uint         `json:"folder_id"`
	FolderName string        `json:"folder_name"`
	Path       string        `json:"path"`
	Files      []FileEntry   `json:"files"`
	Folders    []FolderEntry `json:"folders"`
}

// FileEntry is a single file item in a ListResponse.
type FileEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	S3Key     string `json:"s3_key"`
	CreatedAt string `json:"created_at"`
}

// FolderEntry is a single folder item in a ListResponse.
type FolderEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// cdCmd changes the working directory within the active box. The new path is
// saved in Redis so every subsequent command uses it without re-specifying it.
// Supports "cd .." (go up one level), absolute paths ("/some/path"), and
// relative paths ("sub-folder").
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
		RDB, err := cache.NewRedisClient()
		var newPath string

		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}

		IsLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			fmt.Println("You are not logged in.")
			return nil
		}

		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		currentPath, _ := cache.GetCurrentPath(RDB)

		// Resolve the new path from the argument.
		if len(args) == 0 {
			// "nim cd" with no arguments goes to the box root.
			newPath = ""
		} else {
			targetPath := args[0]
			if strings.HasPrefix(targetPath, "/") {
				// Absolute path: strip the leading slash and use as-is.
				newPath = strings.TrimPrefix(targetPath, "/")
			} else if targetPath == ".." {
				// Go up one level by removing the last path segment.
				if currentPath == "" {
					newPath = "" // already at root
				} else {
					lastSlash := strings.LastIndex(currentPath, "/")
					if lastSlash == -1 {
						newPath = ""
					} else {
						newPath = currentPath[:lastSlash]
					}
				}
			} else if strings.Contains(targetPath, "..") {
				// Handle paths like "../sibling" or "foo/../bar" by joining with
				// the current path and letting path.Clean resolve the ".." segments.
				combined := path.Join(currentPath, targetPath)
				newPath = path.Clean(combined)
				if newPath == "." {
					newPath = ""
				}
				// Guard against escaping the box root (e.g. "../../../../etc").
				if strings.HasPrefix(newPath, "..") {
					newPath = ""
				}
			} else {
				// Relative path: append to current.
				if currentPath == "" {
					newPath = targetPath
				} else {
					newPath = currentPath + "/" + targetPath
				}
			}
		}

		// Normalise: trim leading/trailing slashes and collapse any double slashes.
		newPath = strings.Trim(newPath, "/")
		for strings.Contains(newPath, "//") {
			newPath = strings.ReplaceAll(newPath, "//", "/")
		}

		if err := cache.SetCurrentPath(RDB, newPath); err != nil {
			return fmt.Errorf("failed to save path: %w", err)
		}

		if newPath == "" {
			fmt.Printf("%s/\n", CurrentBox)
		} else {
			fmt.Printf("%s/%s\n", CurrentBox, newPath)
		}

		return nil
	},
}

// pwdCmd prints the current working directory (box name + path) so the user
// always knows where they are without running a full "ls".
var pwdCmd = &cobra.Command{
	Use:   "pwd",
	Short: "Print current directory path",
	Long:  `Display the current working directory path within your box.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}

		IsLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			fmt.Println("You are not logged in.")
			return nil
		}

		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		CurrentPath, _ := cache.GetCurrentPath(RDB)

		if CurrentPath == "" {
			fmt.Printf("%s/\n", CurrentBox)
		} else {
			fmt.Printf("%s/%s\n", CurrentBox, CurrentPath)
		}

		return nil
	},
}

// lsCmd lists the contents of the current or a specified directory by querying
// the server. It is the path-aware version of fileListCmd — it resolves the
// target path from the session's current path combined with an optional argument.
var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List contents of current or specified directory",
	Long:  `List files and folders in the current directory or a specified path.`,
	Args:  cobra.MaximumNArgs(1),
	Example: `nim ls              # List current directory
nim ls myfolder     # List contents of myfolder
nim ls /some/path   # List contents of absolute path`,
	RunE: func(cmd *cobra.Command, args []string) error {
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
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

		currentPath, _ := cache.GetCurrentPath(RDB)

		// Determine the target path: either the session's current path, or an
		// argument (absolute overrides, relative is appended to current).
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

		endpoint := fmt.Sprintf(
			config.BaseURL+"/v1/api/folders?box_name=%s&path=%s",
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

		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
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

		var listing ListResponse
		if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Print the path header then folders before files (matching Unix ls convention).
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
