package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"wd-reader/internal/epubexport"
)

func TestExportMergesPagesAndNormalizesText(t *testing.T) {
	pages := map[string]string{
		"/txt/7.html": `<title>示例txt下载_测试_科幻小说_久久小说网</title>
			<article><h3>《示例》</h3><div class="body detail">
			<div class="pic"><img src="/cover.png"></div><div class="kv">
			<p><strong>书籍作者：</strong></p><p><strong>书籍分类：</strong>科幻小说</p>
			<p><strong>写作进度：</strong><span class="sfwj">已完结</span></p></div></div></article>
			<article class="intro-panel"><div class="desc"><p>示例 &amp; 简介</p></div></article>`,
		"/read/7/": `<ul class="chapter-list">
			<li><a href="/read/12/7/1.html">第一章 开篇</a></li>
			<li><a href="/read/12/7/2.html">第一章 开篇(2/2)</a></li>
			<li><a href="/read/12/7/3.html">第二章 加更（2/3）</a></li>
			<li><a href="/read/12/7/4.html">第三章 字数（2613/2000）</a></li>
			<li><a href="/read/12/7/5.html">第四章 进度（1/3）</a></li></ul>`,
		"/read/12/7/1.html": `<div id="view_content_txt"><p>第一章&nbsp;开篇</p><p>第一章 开篇</p>
			<p>&amp;ldquo;甲&amp;rdquo;与&#x4E59;<br>&amp;amp;、&lt;书&gt;、&copy;</p></div>`,
		"/read/12/7/2.html": `<div id="view_content_txt"><p>第一章 开篇(2/2)</p><p>&amp;#39;续页&amp;#39;</p></div>`,
		"/read/12/7/3.html": `<div id="view_content_txt"><p>第二章 加更（2/3）</p><p>正文二</p></div>`,
		"/read/12/7/4.html": `<div id="view_content_txt"><p>正文三</p></div>`,
		"/read/12/7/5.html": `<div id="view_content_txt"><p>正文四</p></div>`,
		"/cover.png":        "",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if page, ok := pages[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, page)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource(0)
	src.base, _ = url.Parse(server.URL)
	b, err := src.details(context.Background(), "7")
	if err != nil || b.Author != "测试" {
		t.Fatalf("details author = %q, err = %v", b.Author, err)
	}
	out := t.TempDir()
	if err := run(context.Background(), src, options{out: out, id: "7"}); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(filepath.Join(out, "科幻小说", "示例 [7].epub"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if slices.ContainsFunc(archive.File, func(f *zip.File) bool { return f.Name == "EPUB/chapter-0005.xhtml" }) {
		t.Fatal("continuation page was emitted as a separate chapter")
	}
	last, err := archive.Open("EPUB/chapter-0004.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	lastData, err := io.ReadAll(last)
	last.Close()
	if err != nil || !strings.Contains(string(lastData), "第四章 进度（1/3）") {
		t.Fatalf("unpaired fraction was not preserved in title: %s, %v", lastData, err)
	}
	f, err := archive.Open("EPUB/chapter-0001.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Body struct {
			Heading    string   `xml:"h1"`
			Paragraphs []string `xml:"p"`
		} `xml:"body"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	want := []string{"“甲”与乙", "&、<书>、©", "'续页'"}
	if doc.Body.Heading != "第一章 开篇" || !slices.Equal(doc.Body.Paragraphs, want) {
		t.Fatalf("chapter body = %+v, want paragraphs %v", doc.Body, want)
	}
	txtOnlyOut := t.TempDir()
	if err := run(context.Background(), src, options{out: txtOnlyOut, id: "7", txtOnly: true}); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(txtOnlyOut); err != nil || len(entries) != 0 {
		t.Fatalf("TXT-only export should skip books with no TXT: %v, %v", entries, err)
	}
}

func TestDownloadTXTResolvesSameSiteLink(t *testing.T) {
	const downloadPage = "/txt/dl-12-7.html"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case downloadPage:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<a href="https://example.invalid/e/DownSys/doaction.php?id=7&amp;pathid=0">外站</a>
				<a href="/e/DownSys/doaction.php?id=8&amp;pathid=0">另一册</a>
				<a href="/e/DownSys/doaction.php?id=7&amp;pathid=1">ZIP</a>
				<a href="/e/DownSys/doaction.php?id=7&amp;pathid=0&amp;pass=abc">TXT</a>`)
		case "/e/DownSys/doaction.php":
			if q := r.URL.Query(); q.Get("id") != "7" || q.Get("pathid") != "0" ||
				q.Get("pass") != "abc" || r.Referer() != "http://"+r.Host+downloadPage {
				http.Error(w, "unexpected TXT request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			fmt.Fprint(w, "示例正文")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource(0)
	src.base, _ = url.Parse(server.URL)
	text, err := src.downloadTXT(context.Background(), book{id: "7", download: downloadPage})
	if err != nil || text != "示例正文" {
		t.Fatalf("downloaded TXT = %q, error = %v", text, err)
	}
}

func TestExportDefersBookOnDownloadRateLimit(t *testing.T) {
	const downloadPage = "/txt/dl-12-7.html"
	requests := make(chan string, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/read/7/":
			fmt.Fprint(w, `<ul class="chapter-list"><li><a href="/read/12/7/1.html">第一章</a></li></ul>`)
		case downloadPage:
			fmt.Fprint(w, `<a href="/e/DownSys/doaction.php?id=7&amp;pathid=0">TXT</a>
				<a href="/e/DownSys/doaction.php?id=7&amp;pathid=1">ZIP</a>`)
		case "/e/DownSys/doaction.php":
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource(0)
	src.base, _ = url.Parse(server.URL)
	out := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b := book{id: "7", download: downloadPage, Book: epubexport.Book{Title: "示例", Category: "测试"}}
	if err := exportBook(ctx, src, options{out: out}, b, nil); !errors.Is(err, errRateLimited) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if entries, err := os.ReadDir(out); err != nil || len(entries) != 0 {
		t.Fatalf("rate-limited book produced output: %v, %v", entries, err)
	}
	if retryIn := time.Until(src.retryAt); retryIn <= 2*time.Second || retryIn > 3*time.Second {
		t.Fatalf("Retry-After was not applied: %s", retryIn)
	}
	waitCtx, stop := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer stop()
	if _, err := src.page(waitCtx, "/read/7/"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("request during cooldown returned %v", err)
	}
	if got := len(requests); got != 2 {
		t.Fatalf("got %d HTTP requests, want 2", got)
	}
}

func TestExportSplitsWholeZIPWhenCatalogDoesNotMatch(t *testing.T) {
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	entry, err := z.Create("示例.txt")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(entry, "开头说明。\n\n第一章 开篇\n\n　　开篇正文。\n\n第二章 转折\n\n　　转折正文。\n\n第三章 完结\n\n　　完结正文。")
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	const downloadPage = "/txt/dl-12-7.html"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case downloadPage:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<a href="/e/DownSys/doaction.php?id=7&amp;pathid=0">TXT</a>
				<a href="/e/DownSys/doaction.php?id=7&amp;pathid=1">ZIP</a>`)
		case "/e/DownSys/doaction.php":
			if r.URL.Query().Get("pathid") == "1" {
				w.Header().Set("Content-Type", "application/zip")
				w.Write(archive.Bytes())
			} else {
				http.NotFound(w, r)
			}
		case "/read/7/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<ul class="chapter-list"><li><a href="/read/12/7/1.html">网页目录里的章名</a></li></ul>`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src := newSource(0)
	src.base, _ = url.Parse(server.URL)
	out := t.TempDir()
	history, err := epubexport.OpenHistory("", out)
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	b := book{id: "7", download: downloadPage, Book: epubexport.Book{Title: "示例", Category: "测试"}}
	if err := exportBook(context.Background(), src, options{out: out, txtOnly: true}, b, history); err != nil {
		t.Fatal(err)
	}
	epub, err := zip.OpenReader(filepath.Join(out, "测试", "示例 [7].epub"))
	if err != nil {
		t.Fatal(err)
	}
	defer epub.Close()
	for i, want := range []string{"开头说明。", "开篇正文。", "转折正文。", "完结正文。"} {
		f, err := epub.Open(fmt.Sprintf("EPUB/chapter-%04d.xhtml", i+1))
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil || !strings.Contains(string(data), want) {
			t.Fatalf("chapter %d = %s, %v", i+1, data, err)
		}
	}
}

func TestSplitTXTMatchesDirectoryAndDecodesEntities(t *testing.T) {
	toc := []chapter{{title: "第一章 开篇"}, {title: "第二章"}}
	text := "《示例》作者：测试\r\n第一章 开篇\r\n第一章&nbsp;开篇\r\n甲 &amp;#x4E59;\r\n第二章\r\n第二章\r\n结束 &amp;gt;\r\n"
	bodies, err := splitTXT(text, "示例", toc)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(bodies[0], []string{"甲 乙"}) || !slices.Equal(bodies[1], []string{"结束 >"}) {
		t.Fatalf("unexpected bodies: %v", bodies)
	}
	if _, err := splitTXT(strings.Split(text, "第二章")[0], "示例", toc); err == nil {
		t.Fatal("accepted TXT without the final chapter")
	}
}

func TestSplitTXTMatchesGeneratedIntroduction(t *testing.T) {
	toc := []chapter{{title: "内容简介"}, {title: "第01章 开篇"}, {title: "第02章 完结"}}
	for _, intro := range []string{
		"书名: 示例[副标题]\n作者: 测试\n简介: 一句话：简介\n文案：第二段\n",
		"《示例》作者：测试\n内容简介：\n一句话：简介\n文案：第二段\n",
		"本书名称: 示例\n本书作者: 测试\n本书简介：一句话：简介\n文案：第二段\n",
	} {
		text := intro + "第01章 开篇\n第01章 开篇\n第一章正文\n第02章 完结\n末章正文"
		bodies, err := splitTXT(text, "示例", toc)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(bodies[0], []string{"一句话：简介", "文案：第二段"}) ||
			!slices.Equal(bodies[1], []string{"第一章正文"}) ||
			!slices.Equal(bodies[2], []string{"末章正文"}) {
			t.Fatalf("unexpected bodies: %v", bodies)
		}
	}
	if _, err := splitTXT("简介：只有简介\n第01章 开篇\n第一章正文", "示例", toc); err == nil {
		t.Fatal("accepted TXT missing an actual chapter")
	}
}

func TestSplitTXTReusesClosedChaptersAndNormalizesTitles(t *testing.T) {
	toc := []chapter{{title: "第一章 开篇"}, {title: "第二章 AMX教练攻击机"},
		{title: "第三章 ：尾声"}, {title: "第四章 番外"}}
	text := "第一章 开篇\n甲\n第二章 AMX教练\\攻击机\n乙\n第三章 ：<br/>尾声\n丙\n"
	bodies, err := splitTXT(text, "示例", toc)
	if err == nil || len(bodies) != len(toc) {
		t.Fatalf("expected partial chapters: %v, %v", bodies, err)
	}
	if !slices.Equal(bodies[0], []string{"甲"}) || !slices.Equal(bodies[1], []string{"乙"}) ||
		bodies[2] != nil || bodies[3] != nil {
		t.Fatalf("unexpected partial chapters: %v", bodies)
	}
}

func TestPrepareTXTUsesOriginalHeadingsForGeneratedSections(t *testing.T) {
	toc := []chapter{{title: "前言 第1节"}, {title: "前言 第2节"}, {title: "前言"}, {title: "第一章"}}
	text := "书名：示例\n简介：说明\n前言\n前言正文\n第一章\n第一章正文"
	chapters, bodies, err := prepareTXT(text, "示例", toc)
	if err != nil || len(chapters) != 3 || chapters[1].title != "前言" || !slices.Equal(bodies[1], []string{"前言正文"}) {
		t.Fatalf("unexpected merged chapters: %v, %v, %v", chapters, bodies, err)
	}

	text = "书首\n☆.第1章：开篇\n甲\n☆.第２章:中篇\n乙\n☆.第3章：尾声\n丙"
	chapters, bodies, err = prepareTXT(text, "示例", toc[:2])
	if err != nil || len(chapters) != 4 || chapters[2].title != "第２章:中篇" ||
		!slices.Equal(bodies[0], []string{"书首"}) || !slices.Equal(bodies[3], []string{"丙"}) {
		t.Fatalf("unexpected original chapters: %v, %v, %v", chapters, bodies, err)
	}
}

func TestPrepareTXTRecoversSyntheticPrefaceAndStoryCollection(t *testing.T) {
	toc := []chapter{{title: "前言 第1节"}, {title: "前言 第2节"}, {title: "第17章"}}
	chapters, bodies, err := prepareTXT("简介：简介正文\n开头正文\n第 17 章\n第十七章正文", "示例", toc)
	if err != nil || len(chapters) != 2 || chapters[0].title != "前言" ||
		!slices.Equal(bodies[0], []string{"简介正文", "开头正文"}) {
		t.Fatalf("unexpected preface: %v, %v, %v", chapters, bodies, err)
	}

	text := "书籍简介\n\n简介段落。\n\n\n\n故事甲\n\n甲的正文。\n(完)\n\n\n\n故事乙\n\n乙的正文。\n（完）"
	chapters, bodies, err = prepareTXT(text, "示例", toc[:2])
	if err != nil || len(chapters) != 3 || chapters[1].title != "故事甲" || chapters[2].title != "故事乙" ||
		!slices.Equal(bodies[2], []string{"乙的正文。", "（完）"}) {
		t.Fatalf("unexpected collection: %v, %v, %v", chapters, bodies, err)
	}
}

func TestPrepareTXTUsesWholeBookForGeneratedOrEmptyCatalog(t *testing.T) {
	text := "示例\n作者\n文案\n第一段正文\n结尾正文"
	for _, toc := range [][]chapter{nil, {{title: "示例 第1节"}, {title: "示例 第2节"}}} {
		chapters, bodies, err := prepareTXT(text, "示例", toc)
		if err != nil || len(chapters) != 1 || chapters[0].title != "示例" ||
			len(bodies) != 1 || !slices.Equal(bodies[0], strings.Split(text, "\n")) {
			t.Fatalf("whole book = %v, %v, %v", chapters, bodies, err)
		}
	}
}
