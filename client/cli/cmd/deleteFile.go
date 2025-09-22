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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)

		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error deleting file: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to delete file: %s — %s", resp.Status, string(errBody))
		}
		fmt.Println("File deleted successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteFileCmd)
	deleteFileCmd.Flags().StringVarP(&deleteFilePathFlag, "file", "f", "", "Path to file to delete (required)")
	deleteFileCmd.MarkFlagRequired("file")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteFileCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteFileCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
