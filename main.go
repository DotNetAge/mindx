package main

import (
	"os"

	"github.com/DotNetAge/mindx/cmd"
)

func main() {
	cmd.RuntimeFS = runtimeFS
	cmd.AppIconFS = appIconFS
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
