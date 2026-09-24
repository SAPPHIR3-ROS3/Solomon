package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// assets is a fallback for native builds until the production GUI bundle is
// wired into the single Solomon binary.
//
//go:embed all:assets
var assets embed.FS

//go:embed assets/icon.png
var appIcon []byte

func main() {
	configureDesktopModelLister()
	if err := wails.Run(&options.App{
		Title:  "Solomon",
		Width:  1280,
		Height: 840,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 23, G: 25, B: 27, A: 1},
		Bind: []interface{}{
			&DesktopBridge{},
			&ServerBridge{},
		},
		Mac: &mac.Options{
			// Keep the title bar hidden while reserving the native inset area for
			// macOS traffic lights.
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "Solomon",
		},
	}); err != nil {
		log.Fatal(err)
	}
}
