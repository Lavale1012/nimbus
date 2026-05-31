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
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type ProgressWriter struct {
	Writer io.Writer
	Bar    *progressbar.ProgressBar
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	if n > 0 {
		pw.Bar.Add(n)
	}
	return n, err
}

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

		IsLoggedIn, err := cache.SessionExists(RDB)
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

		CurrentBox, err := cache.GetBoxName(RDB)
		if err != nil {
			return fmt.Errorf("failed to get current box from cache: %w", err)
		}
		if CurrentBox == "" {
			return fmt.Errorf("no current box set, please set it using 'nim cb [box-name]'")
		}

		jwtToken, err := cache.GetAuthToken(RDB)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		// Step 1: request a presigned GET URL from the server
		presignEndpoint := fmt.Sprintf(
			"http://nim.test/v1/api/files/presign-download?box_name=%s&key=%s",
			url.QueryEscape(CurrentBox),
			url.QueryEscape(keyFlag),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		presignReq, err := http.NewRequestWithContext(ctx, http.MethodGet, presignEndpoint, nil)
		if err != nil {
			return fmt.Errorf("build presign request: %w", err)
		}
		presignReq.Header.Set("Authorization", "Bearer "+jwtToken)

		client := &http.Client{Timeout: 15 * time.Second}
		presignResp, err := client.Do(presignReq)
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

		// Step 2: GET the file directly from S3 using the presigned URL
		downloadCtx, downloadCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer downloadCancel()

		getReq, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, presignData.DownloadURL, nil)
		if err != nil {
			return fmt.Errorf("build download request: %w", err)
		}

		getResp, err := client.Do(getReq)
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
			return fmt.Errorf("error creating output file: %v", err)
		}
		defer outFile.Close()

		bar := progressbar.DefaultBytes(getResp.ContentLength, "downloading "+filepath.Base(keyFlag))
		progressWriter := &ProgressWriter{Writer: outFile, Bar: bar}

		if _, err = io.Copy(progressWriter, getResp.Body); err != nil {
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
