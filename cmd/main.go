package main

import (
	"mindx/internal/adapters/cli"
	"mindx/internal/config"
)

var (
	Version   string = "dev"
	BuildTime string = ""
	GitCommit string = ""
)

func init() {
	config.SetBuildInfo(Version, BuildTime, GitCommit)
}

func main() {
	cli.Execute()
}
