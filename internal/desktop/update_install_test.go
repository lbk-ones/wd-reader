package desktop

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

type updateProviderStub struct {
	updater.Provider
	download func(context.Context, func(int64, int64)) error
}

func (p updateProviderStub) Download(ctx context.Context, _ *updater.Release, _ io.Writer, progress func(int64, int64)) error {
	return p.download(ctx, progress)
}

func TestUpdateDownloadStallAndCancellation(t *testing.T) {
	provider := &updateDownloadProvider{
		Provider: updateProviderStub{download: func(ctx context.Context, _ func(int64, int64)) error {
			<-ctx.Done()
			return ctx.Err()
		}},
		idleTimeout: 20 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := provider.Download(ctx, nil, io.Discard, nil); !errors.Is(err, errUpdateDownloadStalled) {
		t.Fatalf("stalled download error = %v", err)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if err := provider.Download(cancelled, nil, io.Discard, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled download error = %v", err)
	}
}

func TestUpdateDownloadKeepsReceivingBytes(t *testing.T) {
	const interval = 20 * time.Millisecond
	const chunks = 10
	provider := &updateDownloadProvider{
		Provider: updateProviderStub{download: func(ctx context.Context, progress func(int64, int64)) error {
			for n := int64(1); n <= chunks; n++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
					progress(n, chunks)
				}
			}
			return nil
		}},
		idleTimeout: 5 * interval,
	}
	var received int64
	if err := provider.Download(context.Background(), nil, io.Discard, func(written, _ int64) { received = written }); err != nil {
		t.Fatal(err)
	}
	if received != chunks {
		t.Fatalf("received %d progress updates, want %d", received, chunks)
	}
}

func TestCancelUpdateReleasesInstallationForRetry(t *testing.T) {
	a := NewApp()
	ctx, finish, err := a.updateInstall.begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if finish != nil {
			finish()
		}
	}()
	if _, _, err := a.updateInstall.begin(); err == nil {
		t.Fatal("allowed overlapping installations")
	}
	a.CancelUpdate()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("update context was not cancelled: %v", ctx.Err())
	}
	if _, _, err := a.updateInstall.begin(); err == nil {
		t.Fatal("allowed retry before the cancelled installation finished")
	}
	finish()
	finish = nil
	_, finishRetry, err := a.updateInstall.begin()
	if err != nil {
		t.Fatalf("could not retry after cancellation: %v", err)
	}
	finishRetry()
	a.CancelUpdate() // Cancelling an idle updater is harmless.
}
