package cmd

import (
	"fmt"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/utils/helpers"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of your Nimbus account",
	Long:  `Log out of your Nimbus account by clearing the local session cache.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rdb, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to connect to cache: %w", err)
		}
		defer rdb.Close()

		IsLoggedIn, err := helpers.SessionExists(rdb)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			fmt.Println("You are not logged in.")
			return nil
		}

		err = cache.ClearAuthToken(rdb)
		if err != nil {
			return fmt.Errorf("failed to clear session: %w", err)
		}
		fmt.Println("Successfully logged out.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}
