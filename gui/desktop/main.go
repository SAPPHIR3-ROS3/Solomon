package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// assets is a fallback for native builds until the production GUI bundle is
// wired into the single Solomon binary.
//
//go:embed all:assets
var assets embed.FS

func main() {
	if err := wails.Run(&options.App{
		Title:  "Solomon",
		Width:  1280,
		Height: 840,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 23, G: 25, B: 27, A: 1},
		Mac: &mac.Options{
			// Keep macOS' native traffic lights while the web interface extends
			// into the title-bar area.
			TitleBar: mac.TitleBarHidden(),
		},
	}); err != nil {
		log.Fatal(err)
	}
}
