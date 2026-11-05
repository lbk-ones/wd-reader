package main

import (
	"embed"

	"wd-reader/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

var version = "dev"

func main() {
	desktop.Run(assets, icon, version)
}
