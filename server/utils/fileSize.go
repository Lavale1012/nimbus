package utils

import (
	"fmt"
	"os"
)

func GetFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("could not get file info: %v", err)
	}
	return fileInfo.Size(), nil
}
