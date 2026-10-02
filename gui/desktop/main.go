package main

import (
	"embed"
	"log"

	guibundle "github.com/SAPPHIR3-ROS3/Solomon/v2026/gui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// assets provides the fallback document for Wails development.
//
//go:embed all:assets
var assets embed.FS

//go:embed assets/icon.png
var appIcon []byte

func main() {
	if err := ensureDesktopServer(); err != nil {
		log.Fatal(err)
	}
	bridge, closeProxy, err := startDesktopProxy()
	if err != nil {
		log.Fatal(err)
	}
	defer closeProxy()
	configureDesktopModelLister()
	frontend := guibundle.Assets()
	if !guibundle.Ready() {
		frontend = assets
	}
	if err := wails.Run(&options.App{
		Title:  "Solomon",
		Width:  1280,
		Height: 840,
		AssetServer: &assetserver.Options{
			Assets: frontend,
		},
		BackgroundColour: &options.RGBA{R: 23, G: 25, B: 27, A: 1},
		Bind: []interface{}{
			&DesktopBridge{},
			bridge,
		},
		Mac: &mac.Options{
			// Keep the title bar hidden while reserving the native inset area for
			// macOS traffic lights.
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "solomon",
		},
	}); err != nil {
		log.Fatal(err)
	}
}
