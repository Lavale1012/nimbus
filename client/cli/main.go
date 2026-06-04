// main.go is the entry point for the Nimbus CLI.
// It hands control to the Cobra command tree defined in the cmd package.
package main

import (
	"github.com/nimbus/cli/cli/cmd"
)

func main() {
	cmd.Execute()
}
