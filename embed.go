package main

import "embed"

//go:embed runtime/agents
//go:embed runtime/settings
//go:embed runtime/schemas
//go:embed runtime/skills
//go:embed runtime/data
//go:embed runtime/web
//go:embed runtime/mindx.json
var runtimeFS embed.FS

//go:embed assets/images/mindx.png
var appIconFS embed.FS
