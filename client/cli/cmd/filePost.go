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

	"github.com/nimbus/cli/cli/animations"
	"github.com/nimbus/cli/utils"
	"github.com/spf13/cobra"
)

var filePathFlag string

var filePostCmd = &cobra.Command{
	Use:   "post",
	Short: "Upload a file to the API",
	RunE: func(cmd *cobra.Command, args []string) error {
		stop := make(chan struct{})
		go animations.StartSpinner(stop)
		defer func() {
			close(stop)
			fmt.Println()
		}()

		defaultUploadEndpoint, err := utils.GetEnv("DEFAULT_UPLOAD_PATH")
		if err != nil {
			return fmt.Errorf("error loading .env file: %v", err)
		}
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
		if _, err := io.Copy(part, f); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}

		req, err := http.NewRequest(http.MethodPost, defaultUploadEndpoint, &body)

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

		// stop spinner once the HTTP call returns

		if err != nil {
			return err
		}
		defer resp.Body.Close()
		// respBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("%s\n", resp.Status)
		// fmt.Println(string(respBytes))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
