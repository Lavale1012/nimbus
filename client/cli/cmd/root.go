/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nimbus",
	Short: "A cross-platform command-line interface for cloud file storage and management",
	Long: `Nimbus CLI provides a hierarchical file organization system with direct S3 storage.

Files are organized in a simple structure:
- Boxes → Top-level containers (e.g., "work", "school", "photos")
- Folders → Hierarchical directories within a box
- Files → Individual files stored in S3

Current MVP commands:
  nimbus post --file ./document.pdf    Upload a file to S3
  nimbus get --file <s3-key> -o ./doc  Download a file from S3

Visit https://github.com/your-org/nim-cli for more information.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nimbus.yaml)")
}
