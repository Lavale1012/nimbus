package cmd

import (
	"fmt"

	"github.com/nimbus/cli/cache"
	"github.com/spf13/cobra"
)

var setCurrentBoxCmd = &cobra.Command{
	Use:     "cb <box-name>",
	Short:   "Set the active box for file operations",
	Long:    "Set the active box. All subsequent commands (post, get, ls, cdir) will operate within this box.",
	Args:    cobra.ExactArgs(1),
	Example: `nim cb Home-Box`,
	RunE: func(cmd *cobra.Command, args []string) error {
		boxName := args[0]

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

		if err := cache.SetBoxName(RDB, boxName); err != nil {
			return fmt.Errorf("failed to set current box: %w", err)
		}

		fmt.Printf("Active box set to: %s\n", boxName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCurrentBoxCmd)
}
