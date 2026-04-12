package types

import (
	"io"

	"github.com/schollz/progressbar/v3"
)

// ProgressWriter wraps an io.Writer and updates a progress bar as data is written
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

// ProgressReader wraps an io.Reader and updates a progress bar as data is read
type ProgressReader struct {
	Reader io.Reader
	Bar    *progressbar.ProgressBar
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Bar.Add(n)
	}
	return n, err
}
