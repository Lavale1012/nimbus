package cmd

import (
	"context"
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

var fileMoveCmd = &cobra.Command{
	Use:     "mv",
	Short:   "Move a file to a different folder",
	Example: `nim mv --key users/nim-user-1/boxes/Home-Box/notes.txt --to documents`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		s3Key, _ := cmd.Flags().GetString("key")
		targetPath, _ := cmd.Flags().GetString("to")

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		// Move updates the FolderID foreign key in the database.
		// The S3 key (and therefore the actual object location) does not change.
		endpoint := fmt.Sprintf(
			config.BaseURL+"/v1/api/files/move?box_name=%s&key=%s&target_path=%s",
			url.QueryEscape(currentBox),
			url.QueryEscape(s3Key),
			url.QueryEscape(targetPath),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Moving file...")
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		stop()

		if err != nil {
			return fmt.Errorf("error moving file: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("move failed: %s — %s", resp.Status, string(errBody))
		}

		dest := targetPath
		if dest == "" {
			dest = "box root"
		}
		fmt.Printf("Moved to %s\n", dest)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fileMoveCmd)
	fileMoveCmd.Flags().String("key", "", "S3 key of the file to move (required)")
	fileMoveCmd.Flags().String("to", "", "Target folder path (empty = box root)")
	fileMoveCmd.MarkFlagRequired("key")
}
