package desktop

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Embedding the interface keeps this fake limited to window visibility actions.
type readerWindowStub struct {
	application.Window
	visible   bool
	minimised bool
	actions   []string
}

func (w *readerWindowStub) IsVisible() bool   { return w.visible }
func (w *readerWindowStub) IsMinimised() bool { return w.minimised }
func (w *readerWindowStub) Show() application.Window {
	w.visible = true
	w.actions = append(w.actions, "show")
	return w
}
func (w *readerWindowStub) Focus() { w.actions = append(w.actions, "focus") }
func (w *readerWindowStub) Minimise() application.Window {
	w.minimised = true
	w.actions = append(w.actions, "minimise")
	return w
}
func (w *readerWindowStub) UnMinimise() {
	w.minimised = false
	w.actions = append(w.actions, "unminimise")
}

func TestToggleReaderWindowAfterHiddenStartup(t *testing.T) {
	window := &readerWindowStub{}
	// First press opens the hidden window; later presses hide and restore it.
	toggleReaderWindow(window)
	if !window.visible || window.minimised || !slices.Equal(window.actions, []string{"show", "focus"}) {
		t.Fatalf("first shortcut did not open the hidden reader: %+v", window)
	}
	toggleReaderWindow(window)
	if !window.minimised {
		t.Fatal("second shortcut did not minimise the reader")
	}
	toggleReaderWindow(window)
	if !window.visible || window.minimised || !slices.Equal(window.actions, []string{"show", "focus", "minimise", "unminimise", "show", "focus"}) {
		t.Fatalf("shortcut did not restore the minimised reader: %+v", window)
	}
}

func TestOptionalDesktopScript(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
		skip   bool
	}{
		{http.MethodHead, "/wails/custom.js", true},
		{http.MethodGet, "/wails/custom.js", true},
		{http.MethodGet, "/wails/runtime.js", false},
		{http.MethodGet, "/", false},
		{http.MethodPost, "/wails/custom.js", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			called := false
			handler := optionalDesktopScript(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusAccepted)
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if tc.skip {
				if called || response.Code != http.StatusNoContent || response.Body.Len() != 0 || response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
					t.Fatalf("unexpected optional script response: %v", response)
				}
			} else if !called || response.Code != http.StatusAccepted {
				t.Fatal("request did not reach the normal handler")
			}
		})
	}
}
