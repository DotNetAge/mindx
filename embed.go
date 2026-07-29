package main

import (
	"embed"
	"io/fs"
)

//go:embed runtime/agents
//go:embed runtime/settings
//go:embed runtime/skills
//go:embed runtime/data
//go:embed runtime/mindx.json
var runtimeFS embed.FS

//go:embed assets/images/mindx.png
var appIconFS embed.FS

//go:embed web
var webRootFS embed.FS

// webFS 是去掉 web/ 前缀的嵌入式文件系统，可直接用于 http.FS。
var webFS fs.FS

func init() {
	var err error
	webFS, err = fs.Sub(webRootFS, "web")
	if err != nil {
		panic("failed to create web sub filesystem: " + err.Error())
	}
}
