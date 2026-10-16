package main

import (
	"embed"
	"log/slog"
	"net/http"
	"wd-reader/go/book"
	"wd-reader/go/constant"
	"wd-reader/go/log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

var version = "dev"

func main() {
	logger := log.GetLogger()
	service := NewApp()
	app := application.New(application.Options{
		Name:        constant.APP_NAME,
		Description: "A discreet desktop ebook reader",
		Icon:        icon,
		Logger:      slog.New(slog.NewTextHandler(logger.Writer(), nil)),
		Services:    []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: optionalDesktopScript,
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "e3984e08-28dc-4e3d-b70a-45e961589cdc",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if service.window != nil {
					service.window.UnMinimise()
					service.window.Show()
					service.window.Focus()
					service.window.EmitEvent("launchArgs", data.Args)
				}
			},
		},
		OnShutdown: func() { logger.Info("bye bye ...") },
	})
	service.application = app
	startURL := "/"
	if book.IsMac() || book.IsLinux() {
		// Keep the v2 origin so existing localStorage settings and progress remain addressable.
		startURL = "wails://wails/"
	}
	service.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "reader",
		Title:            "wd-reader",
		Width:            400,
		Height:           690,
		MinWidth:         280,
		MinHeight:        120,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		EnableFileDrop:   true,
		URL:              startURL,
		Windows:          application.WindowsWindow{DisableFramelessWindowDecorations: true, HiddenOnTaskbar: true},
		Mac:              application.MacWindow{Backdrop: application.MacBackdropTransparent},
		Linux:            application.LinuxWindow{WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand},
	})
	service.window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		service.window.EmitEvent("files-dropped", event.Context().DroppedFiles())
	})
	// Wails 3 owns registration and unregisters shortcuts during shutdown.
	if err := app.GlobalShortcut.Register("CmdOrCtrl+Space", func() {
		if service.window.IsMinimised() {
			service.window.UnMinimise()
			service.window.Focus()
		} else {
			service.window.Minimise()
		}
	}); err != nil {
		logger.WithError(err).Warn("could not register boss key")
	}
	book.CheckBooksPath()
	logger.Info("starting wd-reader ", version)
	if err := app.Run(); err != nil {
		logger.Error(err)
	}
}

// The desktop runtime probes this optional server-mode script on every load.
// A non-JavaScript empty response lets it skip loading without a console 404.
// Server mode serves its real script before reaching the asset middleware.
func optionalDesktopScript(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wails/custom.js" && (r.Method == http.MethodHead || r.Method == http.MethodGet) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
