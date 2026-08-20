package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed appicon.png
var icon []byte

func main() {
	// Safety net: clear any stale proxy left by a previous crash (taskkill /F).
	ClearStaleProxyOnStartup()
	// Catch Ctrl+C / logoff / shutdown to clear the proxy on non-graceful exit.
	installCrashHandler()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "snowden.system",
		Width:             1280,
		Height:            800,
		MinWidth:          1100,
		MinHeight:         700,
		Frameless:         false,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 24, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose: func(ctx context.Context) bool {
			return false // сворачиваем в трей, не закрываем
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		OnShutdown: func(ctx context.Context) {
			log.Printf("[main] shutting down, stopping adaptive monitor and engine")
			app.adaptive.Stop()
			if err := app.manager.StopVPN(); err != nil {
				log.Printf("[main] engine shutdown warning: %v", err)
			}
			_ = clearSystemProxy()
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
