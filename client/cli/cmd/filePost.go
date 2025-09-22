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
		endpoint := "http://localhost:8080/v1/api/files"

		if filePathFlag == "" {
			return fmt.Errorf("please provide --file PATH")
		}

		f, err := os.Open(filePathFlag)
		if err != nil {
			return fmt.Errorf("error opening file: %v", err)
		}
		defer f.Close()

		// Get file size for progress bar
		fileInfo, err := f.Stat()
		if err != nil {
			return fmt.Errorf("error getting file info: %v", err)
		}

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

		var progressReader *ProgressReader
		var finalErr error

		// Ensure progress bar is finished on any exit path
		defer func() {
			if bar != nil {
				bar.Finish()
				if finalErr != nil {
					fmt.Println() // Add newline after progress bar on error
				}
			}
		}()

		// Create a progress reader that wraps the body
		progressReader = &ProgressReader{
			Reader: &body,
			bar:    bar,
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, progressReader)
		if err != nil {
			finalErr = fmt.Errorf("build request: %w", err)
			return finalErr
		}
		req.Header.Set("Content-Type", w.FormDataContentType())

		// Add a request-scoped timeout (align with client timeout)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			finalErr = fmt.Errorf("error uploading file: %w", err)
			return finalErr
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			fmt.Printf("✅ Successfully uploaded %s (%d bytes)\n", filepath.Base(filePathFlag), fileInfo.Size())
		} else {
			errBody, _ := io.ReadAll(resp.Body)
			finalErr = fmt.Errorf("upload failed: %s — %s", resp.Status, string(errBody))
			return finalErr
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(filePostCmd)
	filePostCmd.Flags().StringVarP(&filePathFlag, "file", "f", "", "Path to file to upload (required)")
	filePostCmd.MarkFlagRequired("file")
}
