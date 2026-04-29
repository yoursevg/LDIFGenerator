package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/yoursevg/LDIFGenerator/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service := app.NewService()
	err := wails.Run(&options.App{
		Title:  "LDIFGenerator",
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: service.SetContext,
		Bind:      []interface{}{service},
	})
	if err != nil {
		println("error:", err.Error())
	}
}
