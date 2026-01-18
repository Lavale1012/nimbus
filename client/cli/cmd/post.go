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

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/utils/helpers"
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

var (
	destinationFlag string
	filePathFlag    string
)

var filePostCmd = &cobra.Command{
	Use:   "post",
	Short: "Upload a file to the API",
	Long: `Upload a file to the Nimbus storage system.

Example:
  nim post -f myfile.txt -d uploads/myfile.txt`,
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
		// Validate required flags
		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		// endpoint := fmt.Sprintf("http://localhost:8080/v1/api/files?box_name=%s&filePath=%s", CurrentBox, destinationFlag)
		endpoint := fmt.Sprintf("http://nim.local/v1/api/files?box_name=%s&filePath=%s", CurrentBox, destinationFlag)

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

		// Note: user_id and box_id are sent as URL query parameters
		fmt.Printf("📤 Sending box_name: %s (in URL)\n", CurrentBox)

		part, err := w.CreateFormFile("file", filepath.Base(filePathFlag))
		if err != nil {
			return fmt.Errorf("failed to create form file: %w", err)
		}

		// Copy file to multipart form (no progress bar here)
		if _, err := io.Copy(part, f); err != nil {
			return fmt.Errorf("failed to copy file to form: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close multipart writer: %w", err)
		}

		fmt.Printf("📦 Total request size: %d bytes\n", body.Len())

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
		req.ContentLength = int64(body.Len())
		req.Header.Set("Content-Type", w.FormDataContentType())
		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			finalErr = fmt.Errorf("failed to get auth token: %w", err)
			return finalErr
		}
		if jwtToken == "" {
			finalErr = fmt.Errorf("no auth token found, please login first")
			return finalErr
		}
		req.Header.Set("Authorization", "Bearer "+jwtToken)
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
			fmt.Printf("\nSuccessfully uploaded %s (%d bytes)\n", filepath.Base(filePathFlag), fileInfo.Size())
		} else {
			errBody, _ := io.ReadAll(resp.Body)
			fmt.Printf("\nUpload failed: %s\n", resp.Status)
			fmt.Printf("Server response: %s\n", string(errBody))
			finalErr = fmt.Errorf("upload failed: %s — %s", resp.Status, string(errBody))
			return finalErr
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(filePostCmd)
	filePostCmd.Flags().StringVarP(&filePathFlag, "file", "f", "", "Path to file to upload (required)")
	filePostCmd.Flags().StringVarP(&destinationFlag, "destination", "d", "", "Destination path for the uploaded file")

	filePostCmd.MarkFlagRequired("file")

}
