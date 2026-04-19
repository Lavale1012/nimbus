/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/nimbus/cli/cache"
	"github.com/spf13/cobra"
)

// setCurrentBoxCmd represents the setCurrentBox command
var setCurrentBoxCmd = &cobra.Command{
	Use:   "cb",
	Short: "cb [box-name]",
	Long:  `Set the current active box for file operations.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer RDB.Close()
		CurrentBox := args[0]
		if CurrentBox == "" {
			return fmt.Errorf("please provide box name")
		}
		err = cache.SetBoxName(RDB, CurrentBox)
		if err != nil {
			return fmt.Errorf("failed to set current box in cache: %w", err)
		}
		fmt.Printf("Current box set to: %s\n", CurrentBox)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCurrentBoxCmd)
	setCurrentBoxCmd.PersistentFlags().String("foo", "", "A help for foo")
}
