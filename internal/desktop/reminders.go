package desktop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type reminderTimers struct {
	sync.Mutex
	cancel [2]context.CancelFunc
	dialog sync.Mutex
}

func (a *App) SetReminder(kind string, minutes int, language string) error {
	if minutes < 0 || minutes > 720 {
		return fmt.Errorf("reminder interval must be between 0 and 720 minutes")
	}

	var index int
	var title, message string
	switch kind {
	case "water":
		title, message = "喝水提醒", "该喝水了！休息一下，喝杯水吧。"
		if language == "en" {
			title, message = "Water reminder", "Time to drink some water!"
		}
	case "movement":
		index = 1
		title, message = "久坐提醒", "该活动了！起来走一走，伸展一下。"
		if language == "en" {
			title, message = "Movement reminder", "Time to stand up, walk around, and stretch!"
		}
	default:
		return fmt.Errorf("unknown reminder: %s", kind)
	}

	if !application.System.IsDesktop() || a.application == nil {
		return nil
	}
	timers := &a.reminders
	timers.Lock()
	defer timers.Unlock()
	if cancel := timers.cancel[index]; cancel != nil {
		cancel()
		timers.cancel[index] = nil
	}
	if minutes == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	timers.cancel[index] = cancel
	go func() {
		ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				timers.dialog.Lock()
				if ctx.Err() == nil {
					a.application.Dialog.Info().SetTitle(title).SetMessage(message).Show()
				}
				timers.dialog.Unlock()
			}
		}
	}()
	return nil
}

func (a *App) stopReminders() {
	a.reminders.Lock()
	defer a.reminders.Unlock()
	for _, cancel := range a.reminders.cancel {
		if cancel != nil {
			cancel()
		}
	}
}
