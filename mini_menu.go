package main

import (
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// MiniMenuState contains display context only; commands stay in the reader window.
type MiniMenuState struct {
	Language    string `json:"language"`
	BookName    string `json:"bookName"`
	ChapterName string `json:"chapterName"`
	Open        bool   `json:"open"`
}

func miniMenuActionAllowed(id string) bool {
	switch id {
	case "title-bar", "preferences", "next-page", "previous-page", "contents",
		"next-chapter", "previous-chapter", "library", "help", "minimize", "quit", "exit-mini":
		return true
	}
	return false
}

// miniMenuBounds uses logical screen coordinates and avoids taskbars and dock areas.
func miniMenuBounds(anchorX, anchorY int, work application.Rect) application.Rect {
	const margin = 8
	width, height := min(344, max(1, work.Width-2*margin)), min(520, max(1, work.Height-2*margin))
	x, y := anchorX+margin, anchorY+margin
	if y+height > work.Y+work.Height-margin {
		y = anchorY - height - margin
	}
	x = max(work.X+margin, min(x, work.X+work.Width-width-margin))
	y = max(work.Y+margin, min(y, work.Y+work.Height-height-margin))
	return application.Rect{X: x, Y: y, Width: width, Height: height}
}

// OpenMiniMenu opens a reusable native window outside the 120px mini reader.
func (a *App) OpenMiniMenu(clientX, clientY int, language, bookName, chapterName string) error {
	if a.application == nil || a.window == nil {
		return errors.New("reader window is unavailable")
	}
	if language != "en" {
		language = "zh-CN"
	}
	readerX, readerY := a.window.Position()
	readerWidth, readerHeight := a.window.Size()
	anchorX := readerX + max(0, min(clientX, readerWidth))
	anchorY := readerY + max(0, min(clientY, readerHeight))
	screen, err := a.window.GetScreen()
	if err != nil || screen == nil {
		screen = a.application.Screen.GetPrimary()
	}
	bounds := application.Rect{X: anchorX + 8, Y: anchorY + 8, Width: 344, Height: 520}
	if screen != nil {
		bounds = miniMenuBounds(anchorX, anchorY, screen.WorkArea)
	}

	a.miniMenuMu.Lock()
	a.miniMenuState = MiniMenuState{Language: language, BookName: bookName, ChapterName: chapterName, Open: true}
	if a.miniMenuWindow == nil {
		a.miniMenuWindow = a.application.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "mini-reader-menu", Title: "wd-reader · Reading actions",
			URL: a.miniMenuURL, Width: bounds.Width, Height: bounds.Height,
			InitialPosition: application.WindowXY, X: bounds.X, Y: bounds.Y,
			Frameless: true, DisableResize: true, AlwaysOnTop: true, Hidden: true,
			BackgroundColour: application.NewRGB(250, 251, 246),
			Windows:          application.WindowsWindow{HiddenOnTaskbar: true},
		})
		a.miniMenuWindow.OnWindowEvent(events.Common.WindowLostFocus, func(_ *application.WindowEvent) {
			a.CloseMiniMenu()
		})
		a.miniMenuWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
			event.Cancel()
			a.CloseMiniMenu()
		})
	}
	menu, state := a.miniMenuWindow, a.miniMenuState
	a.miniMenuMu.Unlock()
	menu.SetSize(bounds.Width, bounds.Height)
	menu.SetPosition(bounds.X, bounds.Y)
	menu.EmitEvent("mini-menu-context", state)
	menu.Show()
	menu.Focus()
	return nil
}

// GetMiniMenuState also handles the first load before frontend event listeners exist.
func (a *App) GetMiniMenuState() MiniMenuState {
	a.miniMenuMu.Lock()
	defer a.miniMenuMu.Unlock()
	return a.miniMenuState
}

func (a *App) CloseMiniMenu() {
	a.miniMenuMu.Lock()
	wasOpen := a.miniMenuState.Open
	a.miniMenuState.Open = false
	menu := a.miniMenuWindow
	a.miniMenuMu.Unlock()
	if menu != nil && wasOpen {
		menu.Hide()
	}
	if wasOpen && a.window != nil {
		a.window.EmitEvent("mini-menu-closed")
	}
}

// SelectMiniMenuAction closes before sending a validated command back to the reader.
func (a *App) SelectMiniMenuAction(id string) error {
	if !miniMenuActionAllowed(id) {
		return errors.New("unknown reader action")
	}
	a.miniMenuMu.Lock()
	isOpen := a.miniMenuState.Open
	// Claim once so double clicks cannot turn two pages or launch two dialogs.
	a.miniMenuState.Open = false
	menu := a.miniMenuWindow
	a.miniMenuMu.Unlock()
	if !isOpen || a.window == nil {
		return errors.New("reader menu is closed")
	}
	if menu != nil {
		menu.Hide()
	}
	a.window.EmitEvent("mini-menu-closed")
	a.window.Focus()
	a.window.EmitEvent("mini-menu-action", id)
	return nil
}
