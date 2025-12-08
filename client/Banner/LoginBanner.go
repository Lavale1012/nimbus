package banner

import (
	"fmt"

	"github.com/common-nighthawk/go-figure"
)

// term is the terminal font, smslant, speed

func ShowLoginBanner() {
	myFigure := figure.NewColorFigure("Welcome to Nimbus CLI!", "smslant", "blue", true)
	Toplines := figure.NewColorFigure("===================================================================", "term", "cyan", true)
	myFigure2 := figure.NewColorFigure("|| Please enter your login credentials for Nimbus Drive ||", "term", "green", true)
	Bottomlines := figure.NewColorFigure("===================================================================", "term", "cyan", true)
	myFigure.Print()
	fmt.Print("\n")
	Toplines.Print()
	myFigure2.Print()
	Bottomlines.Print()
}
