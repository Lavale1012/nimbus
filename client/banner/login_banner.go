// Package banner renders the ASCII art welcome screen shown before the login prompt.
package banner

import (
	"fmt"

	"github.com/common-nighthawk/go-figure"
)

// ShowLoginBanner prints a coloured ASCII art header to the terminal.
// It uses the go-figure library which renders text using figlet fonts.
func ShowLoginBanner() {
	myFigure := figure.NewColorFigure("Welcome to Nimbus CLI!", "smslant", "blue", true)
	Toplines := figure.NewColorFigure("===============================================", "term", "cyan", true)
	myFigure2 := figure.NewColorFigure("|| Please enter your login credentials ||", "term", "green", true)
	Bottomlines := figure.NewColorFigure("===============================================", "term", "cyan", true)
	myFigure.Print()
	fmt.Print("\n")
	Toplines.Print()
	myFigure2.Print()
	Bottomlines.Print()
}
