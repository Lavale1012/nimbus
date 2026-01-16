/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var CurrentBox string

// setCurrentBoxCmd represents the setCurrentBox command
var setCurrentBoxCmd = &cobra.Command{
	Use:   "cb",
	Short: "cb [box-name]",
	Long:  `Set the current active box for file operations.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		CurrentBox = args[0]
		if CurrentBox == "" {
			return fmt.Errorf("please provide box name")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCurrentBoxCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	setCurrentBoxCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// setCurrentBoxCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
