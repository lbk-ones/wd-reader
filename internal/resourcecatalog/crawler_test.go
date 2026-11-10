package resourcecatalog

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestBundledNovelRules 验证统一默认配置可解析，并保有完整的抓取来源和成品分组。
func TestBundledNovelRules(t *testing.T) {
	config, err := Parse(defaults)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, source := range config.Sources {
		if source.Kind == CrawlerKind {
			count++
		} else if source.Group != defaultSourceGroup {
			t.Errorf("%s is in group %q", source.ID, source.Group)
		}
	}
	if count != 31 || len(config.Sources) != 37 {
		t.Fatalf("got %d crawlers in %d built-in sources", count, len(config.Sources))
	}
}

// bundledCrawler 读取测试使用的内置规则，使测试跟随实际发布的选择器和地址模板。
func bundledCrawler(t *testing.T, id string) Source {
	t.Helper()
	config, err := Parse(defaults)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range config.Sources {
		if source.ID == id {
			return source
		}
	}
	t.Fatalf("missing source %s", id)
	return Source{}
}

// novelTestTransport 允许测试直接为指定地址返回本地响应，同时保留请求中的书源域名。
type novelTestTransport func(*http.Request) (*http.Response, error)

// RoundTrip 将客户端发出的请求交给当前测试的响应函数。
func (transport novelTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

// TestNovelSearchResolveAndDownload 用离线页面验证书名搜索、详情识别和 EPUB 的元数据、封面及章节顺序。
func TestNovelSearchResolveAndDownload(t *testing.T) {
	const home = "https://www.bqg.info"
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	var cover bytes.Buffer
	if err := png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 3, 4))); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: novelTestTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "www.bqg.info" {
			return nil, fmt.Errorf("unexpected host %s", req.URL.Host)
		}
		pages := map[string]string{
			"/search/": `<div class="book-coverlist"><h4><a href="/book/42/">本地小说</a></h4></div>`,
			"/book/42/": `<html lang="zh-TW"><head>
				<meta property="og:novel:book_name" content="本地小说"><meta property="og:novel:author" content="测试作者">
				<meta property="og:novel:category" content="测试分类"><meta property="og:description" content="序言 &amp; 简介">
				<meta property="og:image" content="cover.png"></head>
				<body><dl id="newlist"><dd><a href="1.html">第一章 开篇</a></dd><dd><a href="2.html">第二章 结尾</a></dd></dl></body></html>`,
			"/book/42/1.html":   `<h1 class="readTitle">第一章 开篇</h1><div id="booktxt"><p>正文一。</p><script>广告</script></div><a id="linkNext" href="1_2.html">下一页</a>`,
			"/book/42/1_2.html": `<h1 class="readTitle">第一章 开篇（第2页）</h1><div id="booktxt">正文二。<br>正文三。</div><a id="linkNext" href="2.html">下一章</a>`,
			"/book/42/2.html":   `<h1 class="readTitle">第二章 结尾</h1><div id="booktxt">正文四。(本章完)</div><a id="linkNext" href="./">目录</a>`,
		}
		recorder := httptest.NewRecorder()
		if req.URL.Path == "/book/42/cover.png" {
			if req.Referer() != home+"/book/42/" {
				t.Errorf("cover referer = %q", req.Referer())
			}
			recorder.Write(cover.Bytes())
		} else if body, ok := pages[req.URL.Path]; ok {
			fmt.Fprint(recorder, body)
		} else {
			recorder.WriteHeader(http.StatusNotFound)
		}
		if req.URL.Path == "/search/" && req.URL.Query().Get("searchkey") != "本地小说" {
			t.Errorf("unexpected search query %s", req.URL.RawQuery)
		}
		resp := recorder.Result()
		resp.Request = req
		return resp, nil
	})}
	source := bundledCrawler(t, "sonovel-bqg-info")
	result, err := Search(ctx, client, source, "本地小说")
	if err != nil || len(result.Books) != 1 || result.Books[0].MatchType != "exact" {
		t.Fatalf("search = %+v, %v", result, err)
	}
	book, err := ResolveNovelBook(ctx, client, []Source{source}, home+"/book/42/#catalog")
	if err != nil || book.Title != "本地小说" || book.Author != "测试作者" || book.URL != home+"/book/42/" {
		t.Fatalf("resolve = %+v, %v", book, err)
	}
	if book.Files[0].Name != "本地小说.epub" || book.Files[0].Format != "EPUB" || result.Books[0].Files[0].Format != "EPUB" {
		t.Fatalf("expected EPUB download items: %+v, %+v", book.Files, result.Books[0].Files)
	}
	if err := ValidateCrawlerDownload(source, File{NovelSourceID: source.ID, URL: "https://www.bqg.info.example/book/42/", Name: "小说.epub"}); err == nil {
		t.Fatal("accepted an unrelated host")
	}
	var progress [][2]int64
	filename, err := DownloadCrawler(ctx, client, source, result.Books[0].Files[0], t.TempDir(), func(done, total int64) {
		progress = append(progress, [2]int64{done, total})
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) == 0 || archive.File[0].Name != "mimetype" || archive.File[0].Method != zip.Store {
		t.Fatal("EPUB mimetype must be the first entry and stored without compression")
	}
	for entry, want := range map[string][]string{
		"mimetype": {"application/epub+zip"},
		"EPUB/package.opf": {
			"<dc:title>本地小说</dc:title>", "<dc:creator>测试作者</dc:creator>", "<dc:language>zh-TW</dc:language>",
			"<dc:subject>测试分类</dc:subject>", "<dc:description>序言 &amp; 简介</dc:description>", "<dc:source>" + home + "/book/42/</dc:source>",
		},
		"EPUB/nav.xhtml":          {`href="chapter-0001.xhtml">第一章 开篇`, `href="chapter-0002.xhtml">第二章 结尾`},
		"EPUB/chapter-0001.xhtml": {`xml:lang="zh-TW"`, "<p>正文一。</p>", "<p>正文二。</p>", "<p>正文三。</p>"},
		"EPUB/chapter-0002.xhtml": {"<h1>第二章 结尾</h1>", "<p>正文四。</p>"},
	} {
		data, err := fs.ReadFile(archive, entry)
		if err != nil {
			t.Fatal(err)
		}
		remaining := string(data)
		for _, part := range want {
			_, rest, ok := strings.Cut(remaining, part)
			if !ok {
				t.Errorf("%s missing %q in order", entry, part)
				break
			}
			remaining = rest
		}
	}
	if data, err := fs.ReadFile(archive, "EPUB/cover.png"); err != nil || !bytes.Equal(data, cover.Bytes()) {
		t.Fatalf("EPUB did not include the site cover: %v", err)
	}
	if !slices.Equal(progress, [][2]int64{{0, 2}, {1, 2}, {2, 2}}) {
		t.Fatalf("unexpected progress %v", progress)
	}
}

// TestCrawlerDownloadUsesFiveWorkers 验证任务的协程数优先于书源配置，且章节先后完成不会改变 EPUB 顺序。
func TestCrawlerDownloadUsesFiveWorkers(t *testing.T) {
	const workers = 5
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var ready atomic.Int32
	batchStarted := make(chan struct{})
	lastChapterWritten := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/book/1/" {
			fmt.Fprint(w, `<h1>并发测试</h1><div id="toc">`)
			for i := 1; i <= workers; i++ {
				fmt.Fprintf(w, `<a href="/chapter/%d">第%d章</a>`, i, i)
			}
			fmt.Fprint(w, "</div>")
			return
		}
		if ready.Add(1) == workers {
			close(batchStarted)
		}
		// 等到五个章节同时开始请求才返回正文，验证任务设置可以突破原来的四协程上限。
		select {
		case <-batchStarted:
		case <-r.Context().Done():
			return
		}
		// 先写出第五章的响应，刻意让首章晚于靠后的章节。
		if r.URL.Path == "/chapter/1" {
			select {
			case <-lastChapterWritten:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprintf(w, "<article>正文 %s</article>", r.URL.Path)
		if r.URL.Path == "/chapter/5" {
			close(lastChapterWritten)
		}
	}))
	defer server.Close()
	source := Source{
		ID: "parallel", Kind: CrawlerKind, Enabled: true, HomeURL: server.URL,
		Crawler: &CrawlerRules{Concurrency: 1, TOC: CrawlerTOCRules{Item: "#toc a"}, Chapter: CrawlerChapterRules{Content: "article"}},
	}
	file := File{NovelSourceID: source.ID, URL: server.URL + "/book/1/", Name: "并发测试.epub", Concurrency: workers}
	path, err := DownloadCrawler(ctx, server.Client(), source, file, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for i := 1; i <= workers; i++ {
		entry := fmt.Sprintf("EPUB/chapter-%04d.xhtml", i)
		data, err := fs.ReadFile(archive, entry)
		if err != nil || !strings.Contains(string(data), fmt.Sprintf("<p>正文 /chapter/%d</p>", i)) {
			t.Fatalf("%s contains the wrong chapter: %q, %v", entry, data, err)
		}
	}
}

// TestCrawlerDownloadCanDisableConcurrency 验证串行设置优先于书源的五协程配置。
func TestCrawlerDownloadCanDisableConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstChapterWritten := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/book/1/":
			fmt.Fprint(w, `<h1>串行测试</h1><div id="toc"><a href="/chapter/1">第一章</a><a href="/chapter/2">第二章</a></div>`)
		case "/chapter/1":
			// 等待时间超过相邻请求的限速间隔，错误开启并发时第二章会提前到达。
			select {
			case <-time.After(650 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			fmt.Fprint(w, "<article>第一章正文</article>")
			close(firstChapterWritten)
		case "/chapter/2":
			select {
			case <-firstChapterWritten:
			default:
				t.Error("second chapter started before the first completed")
			}
			fmt.Fprint(w, "<article>第二章正文</article>")
		}
	}))
	defer server.Close()
	source := Source{
		ID: "serial", Kind: CrawlerKind, Enabled: true, HomeURL: server.URL,
		Crawler: &CrawlerRules{Concurrency: 5, TOC: CrawlerTOCRules{Item: "#toc a"}, Chapter: CrawlerChapterRules{Content: "article"}},
	}
	file := File{NovelSourceID: source.ID, URL: server.URL + "/book/1/", Name: "串行测试.epub", Concurrency: 1}
	if _, err := DownloadCrawler(ctx, server.Client(), source, file, t.TempDir(), nil); err != nil {
		t.Fatal(err)
	}
}

// TestSourceHealth 验证正常跳转、验证页、限流和服务不可用分别映射到对应的检测状态。
func TestSourceHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/redirect":
			http.Redirect(w, req, "/reachable", http.StatusFound)
		case "/reachable":
			fmt.Fprint(w, "OK")
		case "/challenge":
			fmt.Fprint(w, "<title>Just a moment...</title>")
		case "/limited":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	for path, want := range map[string]string{
		"/redirect": "available", "/challenge": "blocked", "/limited": "blocked", "/down": "unavailable",
	} {
		t.Run(path, func(t *testing.T) {
			health := CheckSource(context.Background(), server.Client(), Source{Kind: CrawlerKind, HomeURL: server.URL + path})
			if health.Status != want {
				t.Fatalf("got %+v, want %s", health, want)
			}
		})
	}
}

// TestNovelCatalogSelectPaginationAndOnclick 验证下拉分页、隐藏目录项清理和脚本式章节链接的提取。
func TestNovelCatalogSelectPaginationAndOnclick(t *testing.T) {
	const host = "https://www.52bqg.com"
	source := bundledCrawler(t, "sonovel-52bqg-com")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := &http.Client{Transport: novelTestTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/book_12/index_2.html" {
			return nil, fmt.Errorf("unexpected catalog page %s", req.URL.Path)
		}
		w := httptest.NewRecorder()
		fmt.Fprint(w, `<ul class="section-list"><li><a onclick="read_tz(2);">第二章</a></li></ul>`)
		resp := w.Result()
		resp.Request = req
		return resp, nil
	})}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	d := novelDownload{source: source, client: client, tick: ticker.C}
	d.source.Crawler.TOC.URL = ""
	detail, err := parseHTML([]byte(`<style>li:nth-child(1) {display: none}</style>
		<ul class="section-list"><li><a onclick="read_tz(8);">最近更新</a></li></ul>
		<ul class="section-list"><li><a onclick="read_tz(9);">干扰链接</a></li><li><a onclick="read_tz(1);">第一章</a></li></ul>
		<select><option value="index_1.html">第1页</option><option value="index_2.html">第2页</option></select>`))
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse(host + "/book_12/index_1.html")
	chapters, err := d.chapters(ctx, host+"/book_12/", detail, base)
	if err != nil || len(chapters) != 2 || !strings.HasSuffix(chapters[0].target, "/book_12/1.html") ||
		!strings.HasSuffix(chapters[1].target, "/book_12/2.html") {
		t.Fatalf("chapters = %v, %v", chapters, err)
	}
}
