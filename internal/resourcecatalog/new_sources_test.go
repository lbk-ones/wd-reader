package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTMLPOSTSearchEncodesForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("keyboard") != "诡秘之主 & 作者" || r.Form.Get("tbname") != "download" {
				t.Errorf("bad search form: %v", r.Form)
			}
			fmt.Fprint(w, `<a href="/txt/1.html" title="诡秘之主">诡秘之主</a>`)
			return
		}
		fmt.Fprint(w, `<a href="/book.txt">TXT</a>`)
	}))
	defer server.Close()
	s := Source{ID: "post", Name: "POST test source", Kind: HTMLKind, SearchURL: server.URL + "/search", SearchMethod: "POST", SearchForm: map[string]string{"keyboard": "{query}", "tbname": "download"}, BookLinkPattern: `^/txt/[0-9]+[.]html$`, DownloadLinkPattern: `/book[.]txt$`}
	// The full user input is safely posted; no result is a legitimate filtered response for a mismatched author query.
	if _, err := Search(context.Background(), server.Client(), s, "诡秘之主 & 作者"); err != nil {
		t.Fatal(err)
	}
	config := Config{Version: 1, Sources: []Source{s}}
	data, _ := json.Marshal(config)
	if _, err := Parse(data); err != nil {
		t.Fatal(err)
	}
	s.SearchForm = map[string]string{"keyboard": "fixed"}
	config.Sources = []Source{s}
	data, _ = json.Marshal(config)
	if _, err := Parse(data); err == nil {
		t.Fatal("POST without query accepted")
	}
}

func TestFileIndexFiltersAndCachesNames(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `[{"path":"凡人修仙传.txt","size":100,"type":"file"},{"path":"书库/仙工开物.txt","size":200,"type":"file"},{"path":"凡人修仙传续篇.txt","size":100,"type":"file"},{"path":"../凡人修仙传.txt","size":100,"type":"file"},{"path":"凡人修仙传.exe","size":100,"type":"file"},{"path":"folder","size":0,"type":"directory"}]`)
	}))
	defer server.Close()
	source := Source{ID: "index", Kind: FileIndexKind, IndexURL: server.URL, BookURL: "https://example.org/blob/{file}", DownloadURL: "https://example.org/download/{file}", AuthorHints: map[string]string{"凡人修仙传": "忘语"}}
	got, err := Search(context.Background(), server.Client(), source, "凡人修仙传")
	if err != nil || len(got.Books) != 2 || got.Books[0].MatchType != "exact" || got.Books[0].Author != "忘语" {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = Search(context.Background(), server.Client(), source, "仙工开物")
	if err != nil || len(got.Books) != 1 || !strings.Contains(got.Books[0].Files[0].URL, "/download/%E4%B9%A6%E5%BA%93/") {
		t.Fatalf("%+v %v", got, err)
	}
	if requests.Load() != 1 {
		t.Fatal("file index was not cached")
	}
}

type sampleBook struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Genre  string `json:"genre"`
}
type sourceCheck struct {
	SourceID       string `json:"sourceId"`
	Title          string `json:"title"`
	ExpectedAuthor string `json:"expectedAuthor"`
	Status         string `json:"status"`
	Author         string `json:"author,omitempty"`
	URL            string `json:"url,omitempty"`
	Files          int    `json:"files"`
	Error          string `json:"error,omitempty"`
}
type downloadCheck struct {
	SourceID string `json:"sourceId"`
	Title    string `json:"title"`
	Bytes    int64  `json:"bytes"`
}

func TestLiveNewSourceBookList(t *testing.T) {
	if os.Getenv("WD_READER_LIVE_NEW_SOURCES") != "1" {
		t.Skip("opt-in network integration")
	}
	data, err := os.ReadFile("testdata/chinese_novels.json")
	if err != nil {
		t.Fatal(err)
	}
	var samples []sampleBook
	if err = json.Unmarshal(data, &samples); err != nil {
		t.Fatal(err)
	}
	config, err := Parse(defaults)
	if err != nil {
		t.Fatal(err)
	}
	checks := []sourceCheck{}
	downloads := []downloadCheck{}
	for _, s := range config.Sources {
		if s.ID != "jjxs" && s.ID != "hf-novelstory" {
			continue
		}
		t.Run(s.ID, func(t *testing.T) {
			available := 0
			downloaded := false
			for _, sample := range samples {
				result, e := Search(context.Background(), http.DefaultClient, s, sample.Title)
				check := sourceCheck{SourceID: s.ID, Title: sample.Title, ExpectedAuthor: sample.Author, Status: "not_found"}
				if e != nil {
					check.Status = "error"
					check.Error = e.Error()
					checks = append(checks, check)
					t.Logf("%s: query failed", sample.Title)
					continue
				}
				var book *Book
				for i, b := range result.Books {
					if clean(b.Title) == clean(sample.Title) && clean(b.Author) == clean(sample.Author) {
						book = &result.Books[i]
						break
					}
				}
				if book != nil {
					available++
					check.Status = "available"
					check.Author = book.Author
					check.URL = book.URL
					check.Files = len(book.Files)
					t.Logf("%s: matched author and %d file(s)", sample.Title, check.Files)
					chosen := "诡秘之主"
					if s.Kind == FileIndexKind {
						chosen = "仙工开物"
					}
					if sample.Title == chosen {
						ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
						path, e := Download(ctx, http.DefaultClient, book.Files[0], t.TempDir(), nil)
						cancel()
						if e != nil {
							t.Errorf("%s actual download failed: %v", sample.Title, e)
						} else {
							info, _ := os.Stat(path)
							downloads = append(downloads, downloadCheck{SourceID: s.ID, Title: sample.Title, Bytes: info.Size()})
							downloaded = true
							t.Logf("actual download: %d bytes", info.Size())
						}
					}
				} else {
					t.Logf("%s: no exact author/title match", sample.Title)
				}
				checks = append(checks, check)
			}
			t.Logf("verified coverage %d/%d", available, len(samples))
			if available == 0 || !downloaded {
				t.Error("new source must have matching books and a successful actual download")
			}
		})
	}
	if path := os.Getenv("WD_READER_SOURCE_REPORT"); path != "" {
		report := struct {
			CheckedAt string          `json:"checkedAt"`
			Checks    []sourceCheck   `json:"checks"`
			Downloads []downloadCheck `json:"downloads"`
		}{CheckedAt: time.Now().UTC().Format(time.RFC3339), Checks: checks, Downloads: downloads}
		body, _ := json.MarshalIndent(report, "", "  ")
		if err = os.WriteFile(path, body, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
