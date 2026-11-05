package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const autostartDirectoryArg = "--autostart-directory="

func (a *App) AutostartStatus() (string, error) {
	if a.application == nil || !application.System.IsDesktop() ||
		(runtime.GOOS == "linux" && os.Getenv("APPIMAGE") != "") {
		return "unsupported", nil
	}
	enabled, err := a.application.Autostart.IsEnabled()
	if enabled {
		return "enabled", err
	}
	return "disabled", err
}

func (a *App) SetAutostart(enabled bool) (string, error) {
	status, err := a.AutostartStatus()
	if err != nil || status == "unsupported" {
		return status, err
	}
	if enabled {
		opts := application.AutostartOptions{}
		if runtime.GOOS == "windows" {
			dir, err := os.Getwd()
			if err != nil {
				return status, err
			}
			opts.Arguments = []string{autostartDirectoryArg + dir}
		}
		err = a.application.Autostart.EnableWithOptions(opts)
	} else {
		err = a.application.Autostart.Disable()
	}
	if err != nil {
		return status, err
	}
	return a.AutostartStatus()
}

func restoreAutostartDirectory() error {
	if runtime.GOOS == "windows" && len(os.Args) == 2 {
		if dir, ok := strings.CutPrefix(os.Args[1], autostartDirectoryArg); ok && filepath.IsAbs(dir) {
			return os.Chdir(dir)
		}
	}
	return nil
}
