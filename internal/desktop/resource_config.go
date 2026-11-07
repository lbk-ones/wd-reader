package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// ResourceConfigUsesNativeWindow 判断是否具备原生桌面窗口；浏览器预览需使用独立页面。
func (a *App) ResourceConfigUsesNativeWindow() bool {
	return a.application != nil && application.System.IsDesktop()
}

// OpenResourceConfig 创建或复用默认最大化的配置窗口，并将其显示到前台；返回值表示是否使用原生窗口。
func (a *App) OpenResourceConfig() bool {
	// 非桌面环境交由前端打开独立页面，不尝试创建原生窗口。
	if !a.ResourceConfigUsesNativeWindow() {
		return false
	}
	a.resourceConfigMu.Lock()
	defer a.resourceConfigMu.Unlock()
	// 复用同一个窗口，避免重复打开多个编辑器；默认最大化以容纳完整配置。
	if a.resourceConfigWindow == nil {
		a.resourceConfigWindow = a.application.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "resource-config", Title: "wd-reader · 资源配置",
			URL: a.resourceConfigURL, Width: 1200, Height: 850,
			MinWidth: 640, MinHeight: 480, StartState: application.WindowStateMaximised,
			AlwaysOnTop:      true,
			BackgroundColour: application.NewRGB(250, 251, 246),
		})
		a.resourceConfigWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
			// 隐藏而不销毁，保留编辑器中尚未保存的草稿。
			event.Cancel()
			a.resourceConfigWindow.Hide()
		})
	}
	showReaderWindow(a.resourceConfigWindow)
	return true
}

// CloseResourceConfig 隐藏配置窗口并保留未保存的编辑内容；未创建窗口时返回 false。
func (a *App) CloseResourceConfig() bool {
	a.resourceConfigMu.Lock()
	defer a.resourceConfigMu.Unlock()
	if a.resourceConfigWindow == nil {
		return false
	}
	a.resourceConfigWindow.Hide()
	return true
}
