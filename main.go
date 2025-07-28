package main

import (
	"os"
	"github.com/amirhosseinghanipour/nekogo/gui"
	"github.com/amirhosseinghanipour/nekogo/cmd"
)

func main() {
	if len(os.Args) > 1 {
		cmd.Execute()
	} else {
		gui.RunGUI()
	}
} 