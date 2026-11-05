package desktop

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func updateAssetSuffix() string {
	if !application.System.IsDesktop() {
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		return "_windows_" + runtime.GOARCH + ".exe"
	case "darwin":
		return "_darwin_" + runtime.GOARCH + ".zip"
	case "linux":
		exe, err := os.Executable()
		mount, image := os.Getenv("APPDIR"), os.Getenv("APPIMAGE")
		if err == nil && mount != "" && image != "" && strings.HasPrefix(exe, filepath.Clean(mount)+string(filepath.Separator)) {
			if info, err := os.Stat(image); err == nil && info.Mode().IsRegular() {
				return "_linux_" + runtime.GOARCH + ".AppImage"
			}
		}
	}
	return ""
}

func matchUpdateAsset(name, suffix string) bool {
	return suffix != "" && strings.HasPrefix(name, "wdreader-update_") && strings.HasSuffix(name, suffix)
}

var installUpdateMu sync.Mutex

func (a *App) InstallUpdate(expectedVersion string) error {
	if !installUpdateMu.TryLock() {
		return fmt.Errorf("an update is already in progress")
	}
	defer installUpdateMu.Unlock()

	suffix := updateAssetSuffix()
	if _, _, ok := releaseVersion(a.version); !ok || suffix == "" || a.application == nil {
		return fmt.Errorf("in-app updates are unavailable for this build")
	}
	provider, err := github.New(github.Config{
		Repository:    "lbk-ones/wd-reader",
		ChecksumAsset: "wdreader-update" + suffix + ".sha256",
		AssetMatcher: func(_ updater.CheckRequest, assets []github.ReleaseAsset) int {
			for i, asset := range assets {
				if matchUpdateAsset(asset.Name, suffix) {
					return i
				}
			}
			return -1
		},
	})
	if err != nil {
		return err
	}

	u := a.application.Updater
	if err := u.Init(updater.Config{
		CurrentVersion: a.version,
		Providers:      []updater.Provider{provider},
		Window:         updater.WindowNone,
	}); err != nil && !errors.Is(err, updater.ErrAlreadyConfigured) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	release, err := u.Check(ctx)
	if err != nil {
		return err
	}
	if release == nil || release.Channel != "stable" || release.Metadata["github.release.tag"] != expectedVersion {
		return fmt.Errorf("the release changed; check for updates again")
	}
	if release.Verification == nil || len(release.Verification.Digest) != sha256.Size {
		return fmt.Errorf("the update has no valid SHA-256 checksum")
	}
	if err := u.DownloadAndInstall(ctx); err != nil {
		return err
	}
	return u.Restart(ctx)
}
