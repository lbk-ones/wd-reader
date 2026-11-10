package resourcecatalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHTMLDownloadPageKeepsChapterRanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/search":
			fmt.Fprint(w, `<a href="/123/">时停起手，邪神也得给我跪下！</a>`)
		case "/123/":
			fmt.Fprint(w, `<meta http-equiv="content-language" content="zh-CN"><h1><i>100万字</i><a href="/123/">时停起手，邪神也得给我跪下！</a></h1><a>作者：六个葫芦</a><a href="https://advert.example/123/txt.html">广告</a><a href="/123/txt.html#dir">TXT下载</a>`)
		case "/123/txt.html":
			fmt.Fprint(w, `<a href="/txt/?id=123&amp;p=1">时停起手(1-500章).txt</a><a href="/txt/?id=123&amp;p=2">时停起手(501-1000章).txt</a><a href="/txt/?id=123&amp;p=2">重复</a>`)
		default:
			t.Errorf("unexpected page: %s", r.URL.String())
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s := Source{ID: "test", Kind: HTMLKind, SearchURL: server.URL + "/search?q={query}", BookLinkPattern: `^/[0-9]+/$`, DownloadPagePattern: `^/[0-9]+/txt[.]html$`, DownloadLinkPattern: `/txt/[?]id=[0-9]+&p=[0-9]+$`, DownloadFormat: "txt", FileNameFromLink: true, TitlePattern: `(?s)<h1[^>]*>.*?<a[^>]*>([^<]+)</a>`, AuthorPattern: `作者[：:]([^<]+)</a>`}
	got, err := Search(context.Background(), server.Client(), s, "时停起手")
	if err != nil || len(got.Books) != 1 || len(got.Books[0].Files) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	b := got.Books[0]
	if b.Author != "六个葫芦" || !strings.Contains(b.Files[1].Name, "501-1000章") || b.Files[0].Referer != server.URL+"/123/txt.html" {
		t.Fatalf("%+v", b)
	}
}

func TestLiveAdditionalChineseBooks(t *testing.T) {
	if os.Getenv("WD_READER_LIVE_ADDITIONAL") != "1" {
		t.Skip("opt-in network integration")
	}
	config, err := Parse(defaults)
	if err != nil {
		t.Fatal(err)
	}
	var source Source
	for _, s := range config.Sources {
		if s.ID == "sudugu" {
			source = s
		}
	}
	authors := map[string]string{"时停起手": "六个葫芦", "斗破苍穹": "天蚕土豆", "斗罗大陆": "唐家三少", "神印王座": "唐家三少", "仙逆": "耳根"}
	for _, query := range []string{"时停起手", "斗破苍穹", "斗罗大陆", "神印王座", "仙逆"} {
		t.Run(query, func(t *testing.T) {
			currentSource := source
			if query == "神印王座" {
				for _, s := range config.Sources {
					if s.ID == "ixdzs" {
						currentSource = s
					}
				}
			}
			got, err := Search(context.Background(), http.DefaultClient, currentSource, query)
			if err != nil {
				t.Fatal(err)
			}
			var original *Book
			for i, b := range got.Books {
				if (clean(b.Title) == clean(query) || (query == "时停起手" && matchesTitle(b.Title, query))) && clean(b.Author) == clean(authors[query]) {
					original = &got.Books[i]
					break
				}
			}
			if original == nil {
				t.Fatalf("original not found among %d resources", len(got.Books))
			}
			t.Logf("%s: %s · %s: %d files; %s", currentSource.Name, original.Title, original.Author, len(original.Files), original.Files[0].URL)
			if query == "时停起手" {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				path, err := Download(ctx, http.DefaultClient, original.Files[0], t.TempDir(), nil)
				if err != nil {
					t.Fatal(err)
				}
				info, _ := os.Stat(path)
				t.Logf("downloaded chapter-range TXT: %d bytes", info.Size())
			}
		})
	}
}

func TestSequelIsRelatedEvenWhenAuthorMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/search" {
			fmt.Fprint(w, `<a href="/read/1/">神印王座II皓月当空</a>`)
			return
		}
		fmt.Fprint(w, `<meta name="author" content="唐家三少"><a href="/novel.txt">TXT下载</a>`)
	}))
	defer server.Close()
	s := Source{ID: "test", Kind: HTMLKind, SearchURL: server.URL + "/search?q={query}", BookLinkPattern: `^/read/[0-9]+/$`, DownloadLinkPattern: `/novel[.]txt$`, AuthorMeta: "author", AuthorHints: map[string]string{"神印王座": "唐家三少"}}
	got, err := Search(context.Background(), server.Client(), s, "神印王座")
	if err != nil || len(got.Books) != 1 || got.Books[0].MatchType != "related" {
		t.Fatalf("%+v %v", got, err)
	}
}
