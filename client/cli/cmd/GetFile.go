/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// GetFileCmd represents the GetFile command
var GetFileCmd = &cobra.Command{
	Use:   "get",
	Short: "A brief description of your command",

	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("GetFile called")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(GetFileCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// GetFileCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// GetFileCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
