package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wd-reader/go/constant"
	"wd-reader/go/log"
)

//go:embed all:frontend/dist
var assets embed.FS
var version string

//go:embed build/appicon.png
var icon []byte

var ctxStatic context.Context

func main() {

	// recover
	defer func() {
		if r := recover(); r != nil {
			//fmt.Println("Recovered from panic:", r)
			_, err := runtime.MessageDialog(ctxStatic, runtime.MessageDialogOptions{
				Type:          runtime.ErrorDialog,
				Title:         "Error",
				Message:       r.(string),
				Buttons:       []string{"got it"},
				DefaultButton: "",
				CancelButton:  "",
				Icon:          icon,
			})
			if err != nil {
				log.GetLogger().Error(err.Error())
			}
		}
	}()

	mylogger := log.InitLog()
	//fmt.Println(version)

	// Create an instance of the app structure
	app := NewApp()
	server := NewServer()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "wd-reader",
		Width:  400,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		AlwaysOnTop:   true,
		Frameless:     true,
		DisableResize: false,
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableFramelessWindowDecorations: true,
			BackdropType:                      windows.Auto,
		},
		//BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnDomReady: func(ctx context.Context) {
			//runtime.LogInfo(ctx, "app dom ready")
		},
		OnStartup: func(ctx context.Context) {
			ctxStatic = ctx
			app.startup(ctx)
			server.startup(ctx)
			runtime.LogInfo(ctx, "current app version :"+version)
			runtime.LogInfo(ctx, "app started")
		},
		OnShutdown: func(ctx context.Context) {
			runtime.LogInfo(ctx, "bye bye ...")
		},
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			runtime.LogInfo(ctx, "app will close")
			return false
		},
		Bind: []interface{}{
			app,
		},
		Logger:             mylogger,
		LogLevelProduction: logger.INFO,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   fmt.Sprintf("%s %s", constant.APP_NAME, version),
				Message: "WdReader \n\nCopyright © 2024",
				Icon:    icon,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		Linux: &linux.Options{
			ProgramName:         constant.APP_NAME,
			Icon:                icon,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
	})

	if err != nil {
		mylogger.Fatal(err.Error())
		panic(err.Error())
		//fmt.Fatal(err.Error())
	}
}
