/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var deleteFilePathFlag string

// deleteFileCmd represents the deleteFile command
var deleteFileCmd = &cobra.Command{
	Use:   "del",
	Short: "Delete a file",
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint := "http://localhost:8080/v1/api/files/" + deleteFilePathFlag

		if deleteFilePathFlag == "" {
			return fmt.Errorf("please provide --file PATH")
		}

		// Create an indeterminate progress spinner for delete operation
		bar := progressbar.NewOptions(-1,
			progressbar.OptionSetDescription("deleting "+deleteFilePathFlag),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetWidth(15),
			progressbar.OptionThrottle(65*time.Millisecond),
		)

		// Start the spinner
		go func() {
			for {
				bar.Add(1)
				time.Sleep(100 * time.Millisecond)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)

		if err != nil {
			bar.Finish()
			return fmt.Errorf("build request: %w", err)
		}
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			bar.Finish()
			return fmt.Errorf("error deleting file: %w", err)
		}
		defer resp.Body.Close()

		// Stop the spinner
		bar.Finish()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to delete file: %s — %s", resp.Status, string(errBody))
		}
		fmt.Printf("✅ Successfully deleted %s\n", deleteFilePathFlag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteFileCmd)
	deleteFileCmd.Flags().StringVarP(&deleteFilePathFlag, "file", "f", "", "Path to file to delete (required)")
	deleteFileCmd.MarkFlagRequired("file")
}
