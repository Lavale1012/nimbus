package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/nimbus/cli/cache"
	"github.com/nimbus/cli/config"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Create a new Nimbus account",
	RunE: func(cmd *cobra.Command, args []string) error {
		rdb, err := cache.NewRedisClient()
		if err != nil {
			return fmt.Errorf("failed to connect to cache: %w", err)
		}
		defer func() { _ = rdb.Close() }()

		loggedIn, err := cache.SessionExists(rdb)
		if err != nil {
			return fmt.Errorf("failed to check session: %w", err)
		}
		if loggedIn {
			fmt.Println("You are already logged in. Please log out first with: nim logout")
			return nil
		}

		url := config.BaseURL + "/register"
		fmt.Printf("Opening registration page: %s\n", url)
		fmt.Println("Complete the form in your browser, then run: nim login")
		return openBrowser(url)
	},
}

func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}

func init() {
	rootCmd.AddCommand(registerCmd)
}
