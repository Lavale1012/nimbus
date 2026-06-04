package utils

import (
	"fmt"
	"os"
)

// GetFileSize returns the size of a file in bytes.
// It uses os.Stat so the file does not need to be opened.
func GetFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("could not get file info: %v", err)
	}
	return fileInfo.Size(), nil
}
