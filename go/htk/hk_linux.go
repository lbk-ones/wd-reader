//go:build linux

package htk

import "golang.design/x/hotkey"

func New() *hotkey.Hotkey {
	return hotkey.New([]hotkey.Modifier{hotkey.ModCtrl}, hotkey.KeySpace)
}
