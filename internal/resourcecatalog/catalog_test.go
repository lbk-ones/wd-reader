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
	"time"
)

func TestConfigPreservesUserFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "books", "resource-sources.json")
	file, err := Load(path)
	if err != nil || len(file.Sources) == 0 {
		t.Fatalf("%+v %v", file, err)
	}
	changed := strings.Replace(file.Content, file.Sources[0].Name, "我的资源库", 1)
	if err = Save(path, changed); err != nil {
		t.Fatal(err)
	}
	if err = Save(path, `{"version":2}`); err == nil {
		t.Fatal("bad config accepted")
	}
	file, err = Load(path)
	if err != nil || file.Sources[0].Name != "我的资源库" {
		t.Fatal("config overwritten")
	}
}

func TestArchiveOnlyListsActualPublicFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search":
			if r.URL.Query().Get("q") != `title:"红楼梦"` {
				t.Errorf("bad query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"response":{"docs":[{"identifier":"book","title":"红楼梦","creator":["曹雪芹"]},{"identifier":"private","title":"红楼梦私有"},{"identifier":"empty","title":"红楼梦无文件"},{"identifier":"other","title":"无关书籍"}]}}`)
		case "/metadata/book":
			fmt.Fprint(w, `{"metadata":{},"files":[{"name":"红楼梦.epub","size":"123"},{"name":"红楼梦.txt","size":"456"},{"name":"private.epub","private":"true"},{"name":"locked.epub","private":true},{"name":"book_lcp.epub"},{"name":"encrypted.epub","format":"LCP Encrypted EPUB"},{"name":"cover.jpg"}]}`)
		case "/metadata/private":
			fmt.Fprint(w, `{"metadata":{"access-restricted-item":"true"},"files":[{"name":"book.epub"}]}`)
		case "/metadata/empty":
			fmt.Fprint(w, `{"metadata":{},"files":[{"name":"book.pdf"}]}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	source := Source{ID: "test", Name: "测试", Kind: "archive", SearchURL: server.URL + "/search?q=title:{query}", DetailURL: server.URL + "/metadata/{id}", DownloadURL: server.URL + "/download/{id}/{file}", BookURL: server.URL + "/details/{id}"}
	got, err := Search(context.Background(), server.Client(), source, "红楼梦")
	if err != nil || len(got.Books) != 1 || len(got.Books[0].Files) != 2 || got.Skipped != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if got.Books[0].Files[0].Size != 123 || got.Books[0].Author != "曹雪芹" {
		t.Fatalf("bad metadata: %+v", got.Books[0])
	}
}

func TestGutenbergAcquisitionLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>https://example.org/ebooks/11.opds</id><title>Alice</title></entry><entry><id>https://example.org/ebooks/22.opds</id><title>Other</title></entry></feed>`)
			return
		}
		fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><author><name>Lewis Carroll</name></author><link rel="http://opds-spec.org/acquisition" type="application/epub+zip" href="https://example.org/alice.epub"/><link rel="http://opds-spec.org/acquisition" type="application/epub+zip" href="https://example.org/other.epub"/><link rel="http://opds-spec.org/acquisition/buy" type="application/epub+zip" href="https://example.org/buy.epub"/><link rel="http://opds-spec.org/acquisition" type="text/plain" href="https://example.org/alice.txt"/></entry></feed>`)
	}))
	defer server.Close()
	source := Source{ID: "test", Name: "Test", Kind: "gutenberg", SearchURL: server.URL + "/search?q={query}", DetailURL: server.URL + "/ebooks/{id}.opds", BookURL: server.URL + "/ebooks/{id}"}
	got, err := Search(context.Background(), server.Client(), source, "Alice")
	if err != nil || len(got.Books) != 1 || len(got.Books[0].Files) != 2 || got.Books[0].Author != "Lewis Carroll" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestLiveCatalog(t *testing.T) {
	if os.Getenv("WD_READER_LIVE_RESOURCES") != "1" {
		t.Skip("opt-in network integration")
	}
	config, err := Parse(defaults)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range config.Sources {
		t.Run(source.ID, func(t *testing.T) {
			query := "红楼梦"
			if source.Kind == "gutenberg" {
				query = "Alice"
			}
			got, err := Search(context.Background(), http.DefaultClient, source, query)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Books) == 0 {
				t.Fatal("no downloadable files")
			}
			t.Logf("%d resources, first: %s (%d files)", len(got.Books), got.Books[0].Title, len(got.Books[0].Files))
			// Download into test storage only; do not change the user's library.
			if source.Kind == "gutenberg" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				file, err := Download(ctx, http.DefaultClient, got.Books[0].Files[0], t.TempDir(), nil)
				if err != nil {
					t.Fatal(err)
				}
				info, _ := os.Stat(file)
				t.Logf("downloaded %d bytes", info.Size())
			}
		})
	}
}
