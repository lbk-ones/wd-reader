package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
