package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestMiniMenuStaysInsideScreenWorkArea(t *testing.T) {
	for _, work := range []application.Rect{
		{X: 0, Y: 0, Width: 1920, Height: 1040},
		{X: -1440, Y: 32, Width: 1440, Height: 860},
		{X: 1920, Y: -900, Width: 800, Height: 600},
		{X: 0, Y: 0, Width: 300, Height: 400},
	} {
		for _, anchor := range []application.Point{
			{X: work.X, Y: work.Y},
			{X: work.X + work.Width - 1, Y: work.Y + work.Height - 1},
			{X: work.X - 500, Y: work.Y - 500},
			{X: work.X + work.Width + 500, Y: work.Y + work.Height + 500},
		} {
			got := miniMenuBounds(anchor.X, anchor.Y, work)
			if got.X < work.X+8 || got.Y < work.Y+8 || got.X+got.Width > work.X+work.Width-8 || got.Y+got.Height > work.Y+work.Height-8 {
				t.Fatalf("popup %#v escapes work area %#v at anchor %#v", got, work, anchor)
			}
		}
	}
}

func TestMiniMenuOpensAboveBottomEdge(t *testing.T) {
	got := miniMenuBounds(400, 1000, application.Rect{Width: 1920, Height: 1040})
	if got.Height != 520 || got.Y+got.Height >= 1000 {
		t.Fatalf("expected full menu above pointer, got %#v", got)
	}
}

func TestMiniMenuRejectsUnknownOrClosedCommands(t *testing.T) {
	a := NewApp()
	for _, action := range []string{"title-bar", "preferences", "next-page", "previous-page", "contents", "next-chapter", "previous-chapter", "library", "help", "minimize", "quit", "exit-mini"} {
		if !miniMenuActionAllowed(action) {
			t.Fatalf("missing reader action %q", action)
		}
		if err := a.SelectMiniMenuAction(action); err == nil {
			t.Fatalf("closed menu accepted %q", action)
		}
	}
	if miniMenuActionAllowed("unknown") || a.SelectMiniMenuAction("unknown") == nil {
		t.Fatal("unknown action was accepted")
	}
	if err := a.OpenMiniMenu(0, 0, "en", "book.txt", "chapter"); err == nil {
		t.Fatal("opened without a reader window")
	}
	a.miniMenuState = MiniMenuState{Language: "en", BookName: "book.txt", Open: true}
	a.CloseMiniMenu()
	a.CloseMiniMenu()
	if state := a.GetMiniMenuState(); state.Open || state.Language != "en" || state.BookName != "book.txt" {
		t.Fatalf("unexpected state after closing: %#v", state)
	}
}
