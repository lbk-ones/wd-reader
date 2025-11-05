package main

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"wd-reader/go/book"
	"wd-reader/go/constant"
	"wd-reader/go/htk"
	"wd-reader/go/log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

//go:embed all:frontend/dist
var assets embed.FS
var version string

//go:embed build/appicon.png
var icon []byte

var hk *hotkey.Hotkey
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
func clearApp() {
	// ubuntu 下不知道为什么会卡着 卡很久
	if hk != nil && !book.IsLinux() {
		err := hk.Unregister()
		log.GetLogger().Info("htk success unregister")
		if err != nil {
			log.GetLogger().Error(err.Error())
		}
	}
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
		clearApp()
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
			mainthread.Init(func() {
				RegisterHotKey(ctx)
			})

		},
		OnShutdown: func(ctx context.Context) {
			runtime.LogInfo(ctx, "bye bye ...")

		},
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			runtime.LogInfo(ctx, "app will close")
			//dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			//	Type:    runtime.QuestionDialog,
			//	Title:   "Quit?",
			//	Message: "Are you sure you want to quit?",
			//})
			//
			//if err != nil {
			//	return false
			//}
			//return dialog != "Yes"
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
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               true,
				FullSizeContent:            true,
				UseToolbar:                 true,
				HideToolbarSeparator:       true,
			},
			About: &mac.AboutInfo{
				Title:   fmt.Sprintf("%s %s", constant.APP_NAME, version),
				Message: "WdReader \n\nCopyright © 2024",
				Icon:    icon,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
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

// RegisterHotKey boss key
func RegisterHotKey(ctx context.Context) {
	hk = htk.New()
	err := hk.Register()
	if err != nil {
		log.Logger.Error(err)
		return
	}
	log.Logger.Info("register hotkey success")
	go func() {
		for {
			event := <-hk.Keydown()
			log.Logger.Info("hotkey keydown", event)
			if runtime.WindowIsMinimised(ctx) {
				runtime.WindowUnminimise(ctx)
			} else {
				runtime.WindowMinimise(ctx)
			}
		}
	}()
}
