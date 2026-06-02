package cmd

import (
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

var keyFlag string
var outputFileFlag string

type presignDownloadResponse struct {
	DownloadURL string `json:"download_url"`
}

var GetFileCmd = &cobra.Command{
	Use:   "get",
	Short: "Download a file from Nimbus storage",
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

		if outputFileFlag == "" {
			outputFileFlag = filepath.Base(keyFlag)
		}

		currentBox, err := cache.GetBoxName(RDB)
		if err != nil || currentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		// Step 1: request a presigned GET URL from the server
		presignEndpoint := fmt.Sprintf(
			"http://nim.test/v1/api/files/presign-download?box_name=%s&key=%s",
			url.QueryEscape(currentBox),
			url.QueryEscape(keyFlag),
		)

		presignCtx, presignCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer presignCancel()

		presignReq, err := http.NewRequestWithContext(presignCtx, http.MethodGet, presignEndpoint, nil)
		if err != nil {
			return fmt.Errorf("build presign request: %w", err)
		}
		presignReq.Header.Set("Authorization", "Bearer "+jwtToken)

		stop := animations.Spinner("Requesting download URL...")
		client := &http.Client{Timeout: 15 * time.Second}
		presignResp, err := client.Do(presignReq)
		stop()

		if err != nil {
			return fmt.Errorf("error requesting download URL: %w", err)
		}
		defer presignResp.Body.Close()

		if presignResp.StatusCode < 200 || presignResp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(presignResp.Body)
			return fmt.Errorf("failed to get download URL: %s — %s", presignResp.Status, string(errBody))
		}

		var presignData presignDownloadResponse
		if err := json.NewDecoder(presignResp.Body).Decode(&presignData); err != nil {
			return fmt.Errorf("failed to parse presign response: %w", err)
		}

		// Step 2: GET the file directly from S3 with a live progress bar
		downloadCtx, downloadCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer downloadCancel()

		getReq, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, presignData.DownloadURL, nil)
		if err != nil {
			return fmt.Errorf("build download request: %w", err)
		}

		getResp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(getReq)
		if err != nil {
			return fmt.Errorf("error downloading file from S3: %w", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
			errBody, _ := io.ReadAll(getResp.Body)
			return fmt.Errorf("S3 download failed: %s — %s", getResp.Status, string(errBody))
		}

		outFile, err := os.Create(outputFileFlag)
		if err != nil {
			return fmt.Errorf("error creating output file: %w", err)
		}
		defer outFile.Close()

		bar := animations.BytesBar(getResp.ContentLength, "Downloading "+filepath.Base(keyFlag))
		progressWriter := &animations.ProgressWriter{Writer: outFile, Bar: bar}

		if _, err = io.Copy(progressWriter, getResp.Body); err != nil {
			return fmt.Errorf("error saving file: %w", err)
		}

		fmt.Printf("Downloaded %s → %s\n", keyFlag, outputFileFlag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(GetFileCmd)
	GetFileCmd.Flags().StringVarP(&keyFlag, "file", "f", "", "S3 key to download (required)")
	GetFileCmd.Flags().StringVarP(&outputFileFlag, "output", "o", "", "Output filename (optional)")
	GetFileCmd.MarkFlagRequired("file")
}
