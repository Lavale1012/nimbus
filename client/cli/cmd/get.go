

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

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/utils/helpers"
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
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer RDB.Close()
		IsLoggedIn, err := helpers.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !IsLoggedIn {
			return fmt.Errorf("you are not logged in, please login first")
		}

		if keyFlag == "" {
			return fmt.Errorf("please provide --file (the S3 key to download)")
		}
		if outputFileFlag == "" {
			outputFileFlag = filepath.Base(keyFlag)
		}

		endpoint := "http://nim.test/v1/api/files?key=" + keyFlag

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)

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
