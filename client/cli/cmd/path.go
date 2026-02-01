package cmd

import (
	"fmt"
	"path"
	"strings"

	"github.com/nimbus/cli/cache"
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
		// TODO: Implement cd functionality
		// 1. Connect to Redis
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}
		// 2. Check if logged in
		IsLoggedIn, err := helpers.SessionExists(RDB)
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
		// TODO: Implement pwd functionality
		// 1. Connect to Redis

		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("Cache error: %w", err)
		}
		// 2. Check if logged in
		IsLoggedIn, err := helpers.SessionExists(RDB)
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
		// TODO: Implement ls functionality
		// 1. Connect to Redis
		// 2. Check if logged in
		// 3. Check if box is set
		// 4. Get current path from cache
		// 5. Determine target path (current or arg)
		// 6. Query server for directory listing
		// 7. Display results (folders and files)
		return fmt.Errorf("ls command not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(cdCmd)
	rootCmd.AddCommand(pwdCmd)
	rootCmd.AddCommand(lsCmd)
}
