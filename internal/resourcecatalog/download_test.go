package resourcecatalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadRejectsWebpagesAndInvalidFiles(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"novel.txt", "<html>Verify</html>", "application/octet-stream", 200},
		{"novel.txt", "Error", "text/html", 200},
		{"novel.epub", "not an epub", "application/epub+zip", 200},
		{"novel.mobi", "not a mobi", "application/octet-stream", 200},
		{"novel.txt", "", "text/plain", 200},
		{"novel.txt", "denied", "text/plain", 403},
	} {
		t.Run(tc.name+tc.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			dir := t.TempDir()
			if _, err := Download(context.Background(), server.Client(), File{URL: server.URL, Name: tc.name}, dir, nil); err == nil {
				t.Fatal("invalid download accepted")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("failed download left files")
			}
		})
	}
}
func TestDownloadWritesCompleteFileAndProgress(t *testing.T) {
	content := strings.Repeat("第一章\n正文\n", 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, content) }))
	defer server.Close()
	var received int64
	path, err := Download(context.Background(), server.Client(), File{URL: server.URL, Name: "../book.txt"}, t.TempDir(), func(n, total int64) { received = n })
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != content || received != int64(len(content)) || filepath.Base(path) != "book.txt" {
		t.Fatalf("bad file %q %d", path, received)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Download(ctx, server.Client(), File{URL: server.URL, Name: "book.txt"}, t.TempDir(), nil); err == nil {
		t.Fatal("cancelled download succeeded")
	}
}
func TestSafeFileNames(t *testing.T) {
	for _, name := range []string{"../book.txt", `..\book.txt`, "CON", "a:b?c", strings.Repeat("书", 200)} {
		got := SafeFileName(name, ".txt")
		if strings.ContainsAny(got, `/\:<>?*`) || filepath.Ext(got) != ".txt" || len([]rune(got)) > 104 {
			t.Fatalf("unsafe name %q", got)
		}
	}
}
