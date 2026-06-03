package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/cli/animations"
	"github.com/spf13/cobra"
)

var (
	destinationFlag string
	filePathFlag    string
)

type presignUploadResponse struct {
	UploadURL string `json:"upload_url"`
	S3Key     string `json:"s3_key"`
	FileID    uint   `json:"file_id"`
}

var filePostCmd = &cobra.Command{
	Use:   "post",
	Short: "Upload a file to Nimbus storage",
	Long: `Upload a file to the Nimbus storage system.

Example:
nim post -f myfile.txt -d uploads/myfile.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		RDB, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to create Redis client: %w", err)
		}
		defer RDB.Close()

		isLoggedIn, err := cache.SessionExists(RDB)
		if err != nil {
			return fmt.Errorf("failed to check login status: %w", err)
		}
		if !isLoggedIn {
			return fmt.Errorf("you are not logged in, please login first")
		}

		currentBox, err := cache.GetBoxName(RDB)
		if err != nil || currentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil || jwtToken == "" {
			return fmt.Errorf("no auth token found, please login first")
		}

		f, err := os.Open(filePathFlag)
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}
		defer f.Close()

		fileInfo, err := f.Stat()
		if err != nil {
			return fmt.Errorf("error getting file info: %w", err)
		}
		filename := filepath.Base(filePathFlag)

		// Step 1: request a presigned PUT URL from the server
		presignEndpoint := fmt.Sprintf(
			"http://localhost:8080/v1/api/files/presign-upload?box_name=%s&filePath=%s&filename=%s&content_type=application/octet-stream&size=%d",
			url.QueryEscape(currentBox),
			url.QueryEscape(destinationFlag),
			url.QueryEscape(filename),
			fileInfo.Size(),
		)

		presignCtx, presignCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer presignCancel()

		presignReq, err := http.NewRequestWithContext(presignCtx, http.MethodPost, presignEndpoint, nil)
		if err != nil {
			return fmt.Errorf("build presign request: %w", err)
		}
		presignReq.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Requesting upload URL...")
		client := &http.Client{Timeout: 15 * time.Second}
		presignResp, err := client.Do(presignReq)
		stop()

		if err != nil {
			return fmt.Errorf("error requesting upload URL: %w", err)
		}
		defer presignResp.Body.Close()

		if presignResp.StatusCode < 200 || presignResp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(presignResp.Body)
			return fmt.Errorf("failed to get upload URL: %s — %s", presignResp.Status, string(errBody))
		}

		var presignData presignUploadResponse
		if err := json.NewDecoder(presignResp.Body).Decode(&presignData); err != nil {
			return fmt.Errorf("failed to parse presign response: %w", err)
		}

		// Step 2: PUT the file directly to S3 with a live progress bar
		fileData, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		bar := animations.BytesBar(fileInfo.Size(), "Uploading "+filename)
		progressReader := &animations.ProgressReader{
			Reader: bytes.NewReader(fileData),
			Bar:    bar,
		}

		uploadCtx, uploadCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer uploadCancel()

		putReq, err := http.NewRequestWithContext(uploadCtx, http.MethodPut, presignData.UploadURL, progressReader)
		if err != nil {
			return fmt.Errorf("build PUT request: %w", err)
		}
		putReq.ContentLength = fileInfo.Size()
		putReq.Header.Set("Content-Type", "application/octet-stream")

		putResp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(putReq)
		if err != nil {
			return fmt.Errorf("error uploading file to S3: %w", err)
		}
		defer putResp.Body.Close()

		if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(putResp.Body)
			return fmt.Errorf("S3 upload failed: %s — %s", putResp.Status, string(errBody))
		}

		fmt.Printf("Uploaded %s (%d bytes)\n", filename, fileInfo.Size())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(filePostCmd)
	filePostCmd.Flags().StringVarP(&filePathFlag, "file", "f", "", "Path to file to upload (required)")
	filePostCmd.Flags().StringVarP(&destinationFlag, "destination", "d", "", "Destination path for the uploaded file")
	filePostCmd.MarkFlagRequired("file")
}
