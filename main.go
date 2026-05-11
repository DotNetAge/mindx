package main

import (
	"os"

	"github.com/DotNetAge/mindx/cmd"
)

func main() {
	cmd.RuntimeFS = runtimeFS
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
