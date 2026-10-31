package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateVersionComparison(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v1.10.0","draft":false,"prerelease":false}`))
	}))
	defer server.Close()
	for _, test := range []struct {
		current               string
		available, comparable bool
	}{{"v1.9.9", true, true}, {"1.10", false, true}, {"1.10.0", false, true}, {"1.10.0-beta.1", true, true}, {"2.0.0", false, true}, {"dev", false, false}} {
		info, err := fetchUpdate(server.Client(), server.URL, test.current)
		if err != nil || info.Available != test.available || info.Comparable != test.comparable {
			t.Fatalf("%s: %#v %v", test.current, info, err)
		}
	}
}

func TestUpdateRejectsUnavailableOrInvalidRelease(t *testing.T) {
	for _, body := range []string{`{"tag_name":"not-a-version"}`, `{"tag_name":"1.1.0","prerelease":true}`, `not json`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		if _, err := fetchUpdate(server.Client(), server.URL, "1.0.0"); err == nil {
			t.Fatalf("accepted %s", body)
		}
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	if _, err := fetchUpdate(server.Client(), server.URL, "1.0.0"); err == nil {
		t.Fatal("accepted HTTP error")
	}
}
