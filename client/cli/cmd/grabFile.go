/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// grabFileCmd represents the grabFile command
var grabFileCmd = &cobra.Command{
	Use:   "grabFile",
	Short: "A brief description of your command",

	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("grabFile called")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(grabFileCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// grabFileCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// grabFileCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
