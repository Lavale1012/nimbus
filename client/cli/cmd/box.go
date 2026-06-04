package cmd

import (
	"fmt"

	"github.com/nimbus/cli/cache"
	"github.com/spf13/cobra"
)

// setCurrentBoxCmd sets the active box for the current session. All file and
// folder commands (post, get, ls, cdir, etc.) operate inside this box until
// it is changed with another "cb" call.
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

		// Validate the box name against the locally cached list so we don't
		// set an active box that doesn't exist on the server.
		if exists, err := cache.BoxExists(RDB, boxName); err != nil {
			return fmt.Errorf("failed to check box existence: %w", err)
		} else if !exists {
			return fmt.Errorf("box '%s' does not exist", boxName)
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
