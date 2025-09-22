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

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// ProgressWriter wraps an io.Writer and updates a progress bar as data is written
type ProgressWriter struct {
	Writer io.Writer
	bar    *progressbar.ProgressBar
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	if n > 0 {
		pw.bar.Add(n)
	}
	return n, err
}

var keyFlag string
var outputFileFlag string

// GetFileCmd represents the GetFile command
var GetFileCmd = &cobra.Command{
	Use:   "get",
	Short: "A brief description of your command",

	RunE: func(cmd *cobra.Command, args []string) error {
		// stop := make(chan struct{})
		// go animations.StartSpinner(stop)
		// defer func() {
		// 	close(stop)
		// 	fmt.Println()
		// }()
		endpoint := "http://localhost:8080/v1/api/files"
		query := "?key=" + keyFlag

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

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+query, nil)
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

		// Create progress bar for download
		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"downloading "+filepath.Base(keyFlag),
		)

		// Create a progress writer that wraps the file
		progressWriter := &ProgressWriter{
			Writer: outFile,
			bar:    bar,
		}

		// Copy with progress bar tracking actual HTTP response data
		_, err = io.Copy(progressWriter, resp.Body)
		if err != nil {
			return fmt.Errorf("error saving file: %v", err)
		}
		fmt.Printf("✅ Downloaded %s to %s\n", keyFlag, outputFileFlag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(GetFileCmd)
	GetFileCmd.Flags().StringVarP(&keyFlag, "file", "f", "", "S3 key to download (required)")
	GetFileCmd.Flags().StringVarP(&outputFileFlag, "output", "o", "", "Output filename (optional)")
	GetFileCmd.MarkFlagRequired("file")
}
