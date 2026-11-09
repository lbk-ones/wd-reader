package main

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchExportsBooksConcurrently(t *testing.T) {
	var covers atomic.Int32
	var resuming atomic.Bool
	bothDownloading := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wanjie/":
			for _, id := range []string{"7", "7", "8", "9"} {
				fmt.Fprintf(w, `<div class="itemtxt"><h3><a href="/%s/">示例%s</a></h3><p><span>已完结</span><span>科幻小说</span></p></div>`, id, id)
			}
			fmt.Fprint(w, `<div class="page"></div>`)
		case "/7/", "/8/", "/9/":
			id := strings.Trim(r.URL.Path, "/")
			if resuming.Load() && id != "9" {
				t.Errorf("续跑重新请求了已导出的书籍 %s", id)
			}
			fmt.Fprintf(w, `<div class="item"><img src="/cover.png"><div class="itemtxt">
				<h1><a>示例%s</a></h1><p><span>已完结</span><span>科幻小说</span></p><p><a>作者：测试</a></p>
				</div></div><div class="des"><p>简介</p></div><div id="list"><a href="/%s/20.html">开篇</a></div>`, id, id)
		case "/cover.png":
			if covers.Add(1) == 2 {
				close(bothDownloading)
			}
			select {
			case <-bothDownloading:
				fmt.Fprint(w, "\x89PNG\r\n\x1a\n")
			case <-r.Context().Done():
			}
		case "/7/20.html", "/8/20.html", "/9/20.html":
			fmt.Fprint(w, `<div class="con"><p>第一段</p></div><div class="prenext"></div>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource()
	src.base, _ = url.Parse(server.URL)
	out := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options{out: out, startPage: 1, limit: 2, workers: 2, delay: time.Millisecond,
		historyPath: filepath.Join(t.TempDir(), "sudugu-export.jsonl")}
	if err := run(ctx, src, opts); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"示例7 [7].epub", "示例8 [8].epub"} {
		if _, err := os.Stat(filepath.Join(out, "科幻小说", name)); err != nil {
			t.Fatal(err)
		}
	}
	resuming.Store(true)
	opts.limit = 1
	if err := run(ctx, src, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "科幻小说", "示例9 [9].epub")); err != nil {
		t.Fatal(err)
	}
}

func TestExportPaginatedBook(t *testing.T) {
	pages := map[string]string{
		"/7/": `<div class="item"><img src="/cover.png"><div class="itemtxt">
			<h1><i>2万字</i><a>示例 &amp; 试读</a></h1><p><span>已完结</span><span>科幻小说</span></p>
			<p><a>作者：测试</a></p><ul><li><a href="/7/21.html">尾声</a></li></ul></div></div>
			<div class="des"><p>简介 &lt;正文&gt;</p></div>
			<div id="list"><a href="/7/20.html">开篇</a></div>
			<div id="pages"><a href="p-2.html#dir">下一页</a></div>`,
		"/7/p-2.html": `<div id="list"><a href="/7/21.html">尾声</a></div><div id="pages"></div>`,
		"/7/20.html": `<div class="con"><p>开篇</p><p>第一段</p></div>
			<div class="prenext"><a href="/7/20-2.html">下一页</a></div>`,
		"/7/20-2.html": `<div class="con"><p>\&quot;第二段 &amp; \u0027更多\u0027\&quot; C:\books</p></div>
			<div class="prenext"><a href="/7/21.html">下一章</a></div>`,
		"/7/21.html": `<div class="con"><p>尾声</p><p>末章正文</p></div><div class="prenext"></div>`,
		"/cover.png": "\x89PNG\r\n\x1a\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if page, ok := pages[r.URL.Path]; ok {
			fmt.Fprint(w, page)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource()
	src.base, _ = url.Parse(server.URL)
	out := t.TempDir()
	if err := run(context.Background(), src, options{out: out, bookID: "7"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(out, "科幻小说", "示例 & 试读 [7].epub")
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if first := archive.File[0]; first.Name != "mimetype" || first.Method != zip.Store {
		t.Fatalf("invalid first EPUB entry: %s", first.Name)
	}
	for name, want := range map[string]string{
		"mimetype":                "application/epub+zip",
		"EPUB/package.opf":        "<dc:subject>科幻小说</dc:subject>",
		"EPUB/intro.xhtml":        "简介 &lt;正文&gt;",
		"EPUB/nav.xhtml":          `href="chapter-0002.xhtml">尾声`,
		"EPUB/chapter-0001.xhtml": `<p>&#34;第二段 &amp; &#39;更多&#39;&#34; C:\books</p>`,
		"EPUB/chapter-0002.xhtml": "末章正文",
	} {
		f, err := archive.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil || !strings.Contains(string(data), want) {
			t.Fatalf("%s missing %q: %v", name, want, err)
		}
		if name != "mimetype" {
			if err := xml.Unmarshal(data, new(any)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("output should contain only the final EPUB: %v, %v", files, err)
	}
}

func TestSplitVolumeRequiresMatchingCompleteChapters(t *testing.T) {
	toc := []chapter{{title: "开篇"}, {title: "尾声"}}
	volume := "《示例》\r\n作者：测试\r\n\r\n开篇\r\n　　开篇\r\n　　第一段\r\n尾声\r\n　　末章正文\r\n"
	bodies, err := splitVolume([]byte(volume+"更多精彩小说，请访问：速读谷\r\n"), toc)
	if err != nil || len(bodies) != 2 || !slices.Equal(bodies[0], []string{"第一段"}) ||
		!slices.Equal(bodies[1], []string{"末章正文"}) {
		t.Fatalf("splitVolume = %v, %v", bodies, err)
	}
	if _, err := splitVolume([]byte(volume), toc); err == nil {
		t.Fatal("accepted a truncated volume")
	}
	toc[1].title = "另一章"
	if _, err := splitVolume([]byte(volume+"更多精彩小说，请访问：速读谷"), toc); err == nil {
		t.Fatal("accepted a volume with mismatched chapter titles")
	}
}

func TestVolumeBodiesKeepsMatchedChaptersAfterVolumeErrors(t *testing.T) {
	pages := map[string]string{
		"/7/txt.html": `<div id="list">
			<a href="/txt/?id=7&amp;p=1">示例(1-1章).txt</a>
			<a href="/txt/?id=7&amp;p=2">示例(2-4章).txt</a>
			<a href="/txt/?id=7&amp;p=3">示例(5-5章).txt</a>
			<a href="/txt/?id=7&amp;p=4">示例(6-6章).txt</a></div>`,
		"/txt/?id=7&p=1": "开篇&#183;\n　　开篇·\n　　正文1\n更多精彩小说，请访问：速读谷\n",
		"/txt/?id=7&p=2": "第二章\n　　正文2\n异名\n　　异名\n　　待核对\n第四章\n　　正文4\n更多精彩小说，请访问：速读谷\n",
		"/txt/?id=7&p=4": "第六章\n　　正文6\n更多精彩小说，请访问：速读谷\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if page, ok := pages[r.URL.RequestURI()]; ok {
			fmt.Fprint(w, page)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource()
	src.base, _ = url.Parse(server.URL)
	toc := []chapter{{title: "开篇·"}, {title: "第二章"}, {title: "第三章"},
		{title: "第四章"}, {title: "第五章"}, {title: "第六章"}}
	bodies, err := src.volumeBodies(context.Background(), book{id: "7"}, toc)
	if err == nil || len(bodies) != len(toc) {
		t.Fatalf("volumeBodies = %v, %v", bodies, err)
	}
	want := [][]string{{"正文1"}, {"正文2"}, nil, {"正文4"}, nil, {"正文6"}}
	for i, body := range bodies {
		if !slices.Equal(body, want[i]) {
			t.Fatalf("chapter %d = %v, want %v", i+1, body, want[i])
		}
	}
}

func TestVolumeDownloadRetriesInterruptedHTTP2BodyWithHTTP1(t *testing.T) {
	var interrupted atomic.Bool
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 {
			interrupted.Store(true)
			fmt.Fprint(w, "partial")
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		fmt.Fprint(w, "complete volume")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	src := newSource()
	src.base, _ = url.Parse(server.URL)
	src.client = server.Client()
	src.http1Client.Transport.(*http.Transport).TLSClientConfig =
		server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	data, err := src.get(context.Background(), "/txt/?id=7&p=1", 1024)
	if err != nil || !interrupted.Load() || string(data) != "complete volume" {
		t.Fatalf("download = %q, interrupted = %v, error = %v", data, interrupted.Load(), err)
	}
}
