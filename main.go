package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// version 通过 -ldflags "-X main.version=..." 注入，CI/Release 中设置为 git tag。
var version = "dev"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "multi-kimi",
		Width:             430,
		Height:            560,
		MinWidth:          400,
		MinHeight:         400,
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 245, G: 247, B: 251, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "multi-kimi",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				app.showMainWindow()
			},
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
