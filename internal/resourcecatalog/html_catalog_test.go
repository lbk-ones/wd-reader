package resourcecatalog

import (
	"archive/zip"
	"bytes"
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

func TestHTMLCatalogMatchesAliasesAndPrioritizesOriginal(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/search" {
			fmt.Fprint(w, `<a href="https://advert.example/read/999/" title="我有一座恐怖屋">广告</a><a href="/read/2/" title="我有一座恐怖屋同人">我有一座恐怖屋同人</a><a href="/read/1/"><img alt="封面"></a><a href="/read/1/" title="我有一座恐怖屋">我有一座恐怖屋</a><a href="/read/1/p1.html">我有一座恐怖屋第一章</a><a href="/read/3/">其他小说</a>`)
			return
		}
		fmt.Fprintf(w, `<meta property="og:novel:book_name" content="我有一座恐怖屋"><meta property="og:novel:author" content="我会修空调"><a href="/files/book.zip">TXT下载</a><a href="https://advert.example/setup.exe">下载</a><a href="/files/book.zip">重复链接</a>`)
	}))
	defer server.Close()
	source := Source{ID: "html", Name: "测试", Kind: "html", SearchURL: server.URL + "/search?q={query}", BookLinkPattern: `^/read/[0-9]+/$`, DownloadLinkPattern: `^` + server.URL + `/files/.+[.]zip$`, TitleMeta: "og:novel:book_name", AuthorMeta: "og:novel:author", ZipFormat: "txt", Aliases: map[string][]string{"我有一座冒险屋": {"我有一座恐怖屋"}}}
	got, err := Search(context.Background(), server.Client(), source, "我有一座冒险屋")
	if err != nil || len(got.Books) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	first := got.Books[0]
	if first.Title != "我有一座恐怖屋" || first.Author != "我会修空调" || len(first.Files) != 1 {
		t.Fatalf("%+v", first)
	}
	if first.Files[0].ArchiveFormat != "txt" || first.Files[0].Referer != server.URL+"/read/1/" {
		t.Fatalf("%+v", first.Files[0])
	}
}

func TestZIPDownloadExtractsLargestBookWithoutUsingArchivePaths(t *testing.T) {
	var zipped bytes.Buffer
	writer := zip.NewWriter(&zipped)
	for _, entry := range []struct{ name, text string }{{"../../README.txt", "说明"}, {"../../novel.txt", strings.Repeat("第一章\n小说正文\n", 100)}, {"run.exe", "MZ"}} {
		out, _ := writer.Create(entry.name)
		fmt.Fprint(out, entry.text)
	}
	writer.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != "https://example.org/book/1" {
			t.Error("missing download referer")
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipped.Bytes())
	}))
	defer server.Close()
	dir := t.TempDir()
	file, err := Download(context.Background(), server.Client(), File{URL: server.URL, Name: "第一序列.zip", ArchiveFormat: "txt", Referer: "https://example.org/book/1"}, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(file) != dir || filepath.Base(file) != "第一序列.txt" {
		t.Fatalf("unsafe output %s", file)
	}
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "小说正文") {
		t.Fatal("did not extract book text")
	}
}

func TestHTMLCatalogFindsOriginalOnLastPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/search" {
			if r.URL.Query().Get("page") == "15" {
				fmt.Fprint(w, `<a href="/read/1/" title="神秘复苏">神秘复苏</a>`)
			} else {
				fmt.Fprint(w, `<a href="/read/2/">神秘复苏同人</a><a href="/search?q=神秘复苏&amp;page=2">2</a><a href="/search?q=神秘复苏&amp;page=15">末页</a>`)
			}
			return
		}
		fmt.Fprint(w, `<a href="/file.txt">TXT下载</a>`)
	}))
	defer server.Close()
	s := Source{ID: "test", Kind: "html", SearchURL: server.URL + "/search?q={query}", BookLinkPattern: `^/read/[0-9]+/$`, DownloadLinkPattern: `/file[.]txt$`, MaxSearchPages: 2}
	got, err := Search(context.Background(), server.Client(), s, "神秘复苏")
	if err != nil || len(got.Books) != 2 || got.Books[0].Title != "神秘复苏" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestHTMLCatalogChecksAuthorForIdenticalTitles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			id := "wrong"
			if r.URL.Query().Get("q") == "佛前献花" {
				id = "original"
			}
			fmt.Fprintf(w, `<a href="/read/%s/" title="神秘复苏">神秘复苏</a>`, id)
			return
		}
		author := "其他作者"
		if r.URL.Path == "/read/original/" {
			author = "佛前献花"
		}
		fmt.Fprintf(w, `<meta property="author" content="%s"><a href="/file.txt">下载</a>`, author)
	}))
	defer server.Close()
	s := Source{ID: "test", Kind: "html", SearchURL: server.URL + "/search?q={query}", BookLinkPattern: `^/read/[a-z]+/$`, DownloadLinkPattern: `/file[.]txt$`, AuthorMeta: "author", AuthorHints: map[string]string{"神秘复苏": "佛前献花"}}
	got, err := Search(context.Background(), server.Client(), s, "神秘复苏")
	if err != nil || len(got.Books) != 2 || got.Books[0].Author != "佛前献花" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestZIPCannotExtractUnexpectedFormats(t *testing.T) {
	var zipped bytes.Buffer
	writer := zip.NewWriter(&zipped)
	out, _ := writer.Create("run.exe")
	fmt.Fprint(out, "MZ")
	writer.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(zipped.Bytes()) }))
	defer server.Close()
	if _, err := Download(context.Background(), server.Client(), File{URL: server.URL, Name: "book.zip", ArchiveFormat: "txt"}, t.TempDir(), nil); err == nil {
		t.Fatal("ZIP without ebook accepted")
	}
	if err := ValidateDownload(File{URL: server.URL, Name: "book.zip"}); err == nil {
		t.Fatal("ZIP must declare the ebook format")
	}
}

func TestLiveChineseWebNovels(t *testing.T) {
	if os.Getenv("WD_READER_LIVE_CHINESE") != "1" {
		t.Skip("opt-in network integration")
	}
	config, err := Parse(defaults)
	if err != nil {
		t.Fatal(err)
	}
	var source Source
	for _, s := range config.Sources {
		if s.ID == "ixdzs" {
			source = s
		}
	}
	for _, query := range []string{"大周皇族", "第一序列", "神秘复苏", "我有一座冒险屋"} {
		t.Run(query, func(t *testing.T) {
			got, err := Search(context.Background(), http.DefaultClient, source, query)
			if err != nil {
				t.Fatal(err)
			}
			variants := queryVariants(source, query)
			expectedAuthor := map[string]string{"大周皇族": "皇甫奇", "第一序列": "会说话的肘子", "神秘复苏": "佛前献花", "我有一座冒险屋": "我会修空调"}[query]
			var original *Book
			for i, b := range got.Books {
				for _, q := range variants {
					if clean(b.Title) == clean(q) && clean(b.Author) == clean(expectedAuthor) {
						original = &got.Books[i]
					}
				}
			}
			if original == nil {
				t.Fatalf("original book not found among %d results", len(got.Books))
			}
			t.Logf("%s · %s: %s", original.Title, original.Author, original.Files[0].URL)
			if query == "第一序列" {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				path, err := Download(ctx, http.DefaultClient, original.Files[0], t.TempDir(), nil)
				if err != nil {
					t.Fatal(err)
				}
				info, _ := os.Stat(path)
				t.Logf("downloaded and extracted %s (%d bytes)", filepath.Base(path), info.Size())
			}
		})
	}
}
