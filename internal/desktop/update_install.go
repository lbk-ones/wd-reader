package desktop

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

const updateRestartingEvent = "reader:update-restarting"

var errUpdateDownloadStalled = errors.New("下载长时间没有进展，请检查网络后重试")

type updateInstallation struct {
	sync.Mutex
	cancel context.CancelFunc
}

func (i *updateInstallation) begin() (context.Context, func(), error) {
	i.Lock()
	defer i.Unlock()
	if i.cancel != nil {
		return nil, nil, fmt.Errorf("an update is already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel
	return ctx, func() {
		cancel()
		i.Lock()
		i.cancel = nil
		i.Unlock()
	}, nil
}

// CancelUpdate stops checking or downloading before the restart helper is started.
func (a *App) CancelUpdate() {
	a.updateInstall.Lock()
	defer a.updateInstall.Unlock()
	if a.updateInstall.cancel != nil {
		a.updateInstall.cancel()
	}
}

// Keep large downloads alive while bounding requests that stop receiving bytes.
type updateDownloadProvider struct {
	updater.Provider
	idleTimeout time.Duration
}

func (p *updateDownloadProvider) Download(ctx context.Context, release *updater.Release, dst io.Writer, progress func(int64, int64)) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	timer := time.AfterFunc(p.idleTimeout, func() { cancel(errUpdateDownloadStalled) })
	defer timer.Stop()
	err := p.Provider.Download(ctx, release, dst, func(written, total int64) {
		timer.Reset(p.idleTimeout)
		if progress != nil {
			progress(written, total)
		}
	})
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return err
}

func updateInstallError(err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("更新已取消: %w", err)
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return fmt.Errorf("更新超时，请检查网络后重试: %w", err)
	}
	return err
}

func (a *App) InstallUpdate(expectedVersion string) (resultErr error) {
	ctx, finish, err := a.updateInstall.begin()
	if err != nil {
		return err
	}
	defer finish()
	defer func() { resultErr = updateInstallError(resultErr) }()

	suffix := updateAssetSuffix()
	if _, _, ok := releaseVersion(a.version); !ok || suffix == "" || a.application == nil {
		return fmt.Errorf("in-app updates are unavailable for this build")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	defer transport.CloseIdleConnections()
	provider, err := github.New(github.Config{
		Repository:    "lbk-ones/wd-reader",
		HTTPClient:    &http.Client{Transport: transport},
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
		Providers:      []updater.Provider{&updateDownloadProvider{Provider: provider, idleTimeout: 45 * time.Second}},
		Window:         updater.WindowNone,
	}); err != nil && !errors.Is(err, updater.ErrAlreadyConfigured) {
		return err
	}
	checkCtx, cancelCheck := context.WithTimeout(ctx, 30*time.Second)
	release, err := u.Check(checkCtx)
	cancelCheck()
	if err != nil {
		return err
	}
	if release == nil || release.Channel != "stable" || release.Metadata["github.release.tag"] != expectedVersion {
		return fmt.Errorf("the release changed; check for updates again")
	}
	if release.Verification == nil || len(release.Verification.Digest) != sha256.Size {
		return fmt.Errorf("the update has no valid SHA-256 checksum")
	}
	downloadCtx, cancelDownload := context.WithTimeout(ctx, 15*time.Minute)
	defer cancelDownload()
	if err := u.DownloadAndInstall(downloadCtx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	a.application.Event.Emit(updateRestartingEvent)
	return u.Restart(ctx)
}
