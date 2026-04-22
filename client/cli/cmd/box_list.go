package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/types"
	"github.com/spf13/cobra"
)

var ListBoxesCmd = &cobra.Command{
	Use:     "bls",
	Short:   "List all your boxes",
	Long:    "List all boxes in your Nimbus account.",
	Example: `nim bls`,
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

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}
		if jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		req, err := http.NewRequest(http.MethodGet, "http://nim.test/v1/api/boxes", nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error listing boxes: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to list boxes: %s — %s", resp.Status, string(errBody))
		}

		var result types.ListBoxesResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(result.Boxes) == 0 {
			fmt.Println("No boxes found.")
			return nil
		}
		fmt.Print("\n")
		fmt.Printf("%-30s  %s\n", "NAME", "SIZE")
		fmt.Printf("%-30s  %s\n", "----", "----")
		for _, b := range result.Boxes {
			fmt.Printf("%-30s  %s\n", b.Name, formatSize(b.Size))
		}
		fmt.Print("\n")
		return nil
	},
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func init() {
	rootCmd.AddCommand(ListBoxesCmd)
}
