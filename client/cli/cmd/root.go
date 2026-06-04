// Package cmd defines the Cobra command tree for the Nimbus CLI.
// root.go sets up the top-level "nimbus" command; all sub-commands register
// themselves via their own init() functions.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command — running "nim" with no sub-command shows help.
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

// Execute is called by main.go. It runs the appropriate sub-command and exits
// with a non-zero status code if anything goes wrong.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Persistent flags defined here are available to every sub-command.
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nimbus.yaml)")
}
