/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nimbus/cli/utils"
	"github.com/spf13/cobra"
)

var keyFlag string
var outputFileFlag string

// GetFileCmd represents the GetFile command
var GetFileCmd = &cobra.Command{
	Use:   "get",
	Short: "A brief description of your command",

	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, err := utils.GetEnv("DEFAULT_DOWNLOAD_URL")
		if err != nil {
			return fmt.Errorf("error loading .env file: %v", err)
		}

		if endpoint == "" {
			return fmt.Errorf("please provide --file (the S3 key to download)")
		}

		if keyFlag == "" {
			return fmt.Errorf("please provide --file (the S3 key to download)")
		}
		if outputFileFlag == "" {
			outputFileFlag = filepath.Base(keyFlag)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?key="+keyFlag, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error fetching file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to get file: %s — %s", resp.Status, string(errBody))
		}
		outFile, err := os.Create(outputFileFlag)
		if err != nil {
			return fmt.Errorf("error creating output file: %v", err)
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, resp.Body)
		if err != nil {
			return fmt.Errorf("error saving file: %v", err)
		}
		fmt.Printf("✅ Downloaded %s to %s\n", keyFlag, outputFileFlag)
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
	GetFileCmd.Flags().StringVarP(&keyFlag, "file", "f", "", "S3 key to download (required)")
	GetFileCmd.Flags().StringVarP(&outputFileFlag, "output", "o", "", "Output filename (optional)")
	GetFileCmd.MarkFlagRequired("file")
}
