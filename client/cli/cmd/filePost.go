package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// ProgressReader wraps an io.Reader and updates a progress bar as data is read
type ProgressReader struct {
	Reader io.Reader
	bar    *progressbar.ProgressBar
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.bar.Add(n)
	}
	return n, err
}

var filePathFlag string

var filePostCmd = &cobra.Command{
	Use:   "post",
	Short: "Upload a file to the API",
	RunE: func(cmd *cobra.Command, args []string) error {
		// stop := make(chan struct{})
		// go animations.StartSpinner(stop)
		// defer func() {
		// 	close(stop)
		// 	fmt.Println()
		// }()

		endpoint := "http://localhost:8080/v1/api/files"

		if filePathFlag == "" {
			return fmt.Errorf("please provide --file PATH")
		}

		f, err := os.Open(filePathFlag)
		if err != nil {
			return fmt.Errorf("error opening file: %v", err)
		}
		defer f.Close()

		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file", filepath.Base(filePathFlag))
		if err != nil {
			return err
		}

		// Copy file to multipart form (no progress bar here)
		if _, err := io.Copy(part, f); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}

		// Create progress bar for HTTP upload
		bar := progressbar.DefaultBytes(
			int64(body.Len()),
			"uploading "+filepath.Base(filePathFlag),
		)

		// Create a progress reader that wraps the body
		progressReader := &ProgressReader{
			Reader: &body,
			bar:    bar,
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, progressReader)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		// Add a request-scoped timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			fmt.Printf("✅ Successfully uploaded %s\n", filepath.Base(filePathFlag))
		} else {
			fmt.Printf("❌ Upload failed: %s\n", resp.Status)
			return fmt.Errorf("upload failed")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(filePostCmd)
	filePostCmd.Flags().StringVarP(&filePathFlag, "file", "f", "", "Path to file to upload (required)")
	filePostCmd.MarkFlagRequired("file")
}
