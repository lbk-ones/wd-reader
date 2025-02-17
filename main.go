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
	"strings"
	"wd-reader/go/constant"
	"wd-reader/go/log"
)

//go:embed all:frontend/dist
var assets embed.FS
var version string

//go:embed build/appicon.png
var icon []byte

var ctxStatic context.Context

func (a *App) onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	secondInstanceArgs := secondInstanceData.Args
	ctx := a.ctx
	log.GetLogger().Info("user opened second instance", strings.Join(secondInstanceData.Args, ","))
	log.GetLogger().Info("user opened second from", secondInstanceData.WorkingDirectory)
	runtime.WindowUnminimise(ctx)
	runtime.Show(ctx)
	go runtime.EventsEmit(ctx, "launchArgs", secondInstanceArgs)
}
func main() {

	// recover
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("panic error ", r)
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
	//server := NewServer()

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
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "e3984e08-28dc-4e3d-b70a-45e961589cdc",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		BackgroundColour: options.NewRGBA(0, 0, 0, 0),
		OnDomReady: func(ctx context.Context) {
			//runtime.LogInfo(ctx, "app dom ready")
		},
		OnStartup: func(ctx context.Context) {
			ctxStatic = ctx
			app.startup(ctx)
			//server.startup(ctx)
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
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableFramelessWindowDecorations: true,
			BackdropType:                      windows.Auto,
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
