package animations

import (
	"fmt"
	"io"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Spinner starts an indeterminate spinner with the given description and returns
// a stop function. Always call stop — defer it right after calling Spinner.
//
//	stop := animations.Spinner("Creating box...")
//	defer stop()
func Spinner(desc string) func() {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionClearOnFinish(),
	)

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				bar.Add(1)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	return func() {
		close(stop)
		bar.Finish()
		fmt.Print("\r\033[K") // clear spinner line so output prints cleanly
	}
}

// BytesBar creates a determinate progress bar for streaming a known number of bytes.
func BytesBar(total int64, desc string) *progressbar.ProgressBar {
	return progressbar.NewOptions64(total,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Println() }),
		progressbar.OptionSetRenderBlankState(true),
	)
}

// ProgressReader wraps an io.Reader and advances a BytesBar as bytes are read.
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

// ProgressWriter wraps an io.Writer and advances a BytesBar as bytes are written.
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
