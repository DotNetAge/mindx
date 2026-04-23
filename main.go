package main

import (
	"os"

	"github.com/DotNetAge/mindx/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
