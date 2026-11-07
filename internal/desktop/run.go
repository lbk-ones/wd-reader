package desktop

import (
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"wd-reader/internal/book"
	"wd-reader/internal/constant"
	"wd-reader/internal/logs"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func Run(assets fs.FS, icon []byte, version string) {
	if err := restoreAutostartDirectory(os.Args[1:]); err != nil {
		log.Printf("autostart: could not restore books working directory: %v", err)
		return
	}
	logger := logs.GetLogger()
	service := NewApp()
	service.version = version
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
				// A duplicate login launch must not bring the reader to the foreground.
				if isAutostartLaunch(data.Args) {
					return
				}
				if service.window != nil {
					showReaderWindow(service.window)
					service.window.EmitEvent("launchArgs", data.Args)
				}
			},
		},
		OnShutdown: func() {
			service.stopResourceDownloads()
			service.CancelUpdate()
			service.DiscardResourceImport("")
			service.stopReminders()
			logger.Info("bye bye ...")
		},
	})
	service.application = app
	startURL := "/"
	if book.IsMac() || book.IsLinux() {
		// Keep the v2 origin so existing localStorage settings and progress remain addressable.
		startURL = "wails://wails/"
	}
	service.miniMenuURL = startURL + "?view=mini-menu"
	// 主窗口
	service.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "reader",
		Title:            "wd-reader",
		Width:            400,
		Height:           690,
		MinWidth:         280,
		MinHeight:        120,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           isAutostartLaunch(os.Args[1:]),
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
	service.window.OnWindowEvent(events.Common.WindowMinimise, func(_ *application.WindowEvent) {
		service.CloseMiniMenu()
	})
	service.window.OnWindowEvent(events.Common.WindowHide, func(_ *application.WindowEvent) {
		service.CloseMiniMenu()
	})
	service.window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		service.CloseMiniMenu()
		service.window.Minimise()
	})
	if application.System.IsDesktop() {
		showReader := func() { showReaderWindow(service.window) }
		menu := app.NewMenu()
		menu.Add("打开 / Open").OnClick(func(*application.Context) { showReader() })
		menu.AddSeparator()
		menu.Add("退出 / Quit").OnClick(func(*application.Context) { app.Quit() })

		tray := app.SystemTray.New()
		tray.SetIcon(icon)
		tray.SetTooltip("wd-reader")
		tray.SetMenu(menu)
		tray.OnClick(showReader)
	}
	// Wails 3 owns registration and unregisters shortcuts during shutdown.
	if err := app.GlobalShortcut.Register("CmdOrCtrl+Space", func() {
		service.CloseMiniMenu()
		toggleReaderWindow(service.window)
	}); err != nil {
		logger.WithError(err).Warn("could not register boss key")
	}
	book.CheckBooksPath()
	logger.Info("starting wd-reader ", version)
	if err := app.Run(); err != nil {
		logger.Error(err)
	}
}

func toggleReaderWindow(window application.Window) {
	if window.IsMinimised() || !window.IsVisible() {
		showReaderWindow(window)
	} else {
		window.Minimise()
	}
}

func showReaderWindow(window application.Window) {
	if window.IsMinimised() {
		window.UnMinimise()
	}
	window.Show()
	window.Focus()
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
