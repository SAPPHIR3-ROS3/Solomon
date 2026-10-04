package main

import (
	"context"
	_ "embed"
	"log"
	"os/signal"
	"syscall"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	var stopSignals context.CancelFunc
	var cleanupClient func()
	if err := wails.Run(&options.App{
		Title:  "Solomon",
		Width:  1280,
		Height: 840,
		OnStartup: func(ctx context.Context) {
			shutdown, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
			stopSignals = stop
			request, cleanup, err := lifecycle.RegisterClient(ctx, "desktop", nil)
			if err != nil {
				log.Printf("register desktop: %v", err)
				wailsruntime.Quit(ctx)
				return
			}
			cleanupClient = cleanup
			go func() {
				select {
				case <-request:
					wailsruntime.Quit(ctx)
				case <-ctx.Done():
				}
			}()
			go func() { <-shutdown.Done(); wailsruntime.Quit(ctx) }()
		},
		OnShutdown: func(context.Context) {
			if cleanupClient != nil {
				cleanupClient()
			}
			if stopSignals != nil {
				stopSignals()
			}
		},
		AssetServer: &assetserver.Options{
			Handler: serverruntime.DesktopFrontend(),
		},
		BackgroundColour: &options.RGBA{R: 23, G: 25, B: 27, A: 1},
		Bind: []interface{}{
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
