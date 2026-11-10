package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wd-reader/internal/book"
	"wd-reader/internal/resourcecatalog"
)

func waitResourceTask(t *testing.T, a *App, id string) DownloadTask {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, task := range a.ListResourceDownloads() {
			if task.ID == id && (task.Status == "done" || task.Status == "error" || task.Status == "cancelled") {
				return task
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("download task did not finish")
	return DownloadTask{}
}
func TestResourceDownloadImportsIntoLibrary(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, "第一章 开始\n这是一段正文。\n第二章 后续\n这是第二段正文。\n")
	}))
	defer server.Close()
	a := NewApp()
	defer a.stopResourceDownloads()
	task, err := a.StartResourceDownload(resourcecatalog.File{URL: server.URL, Name: "下载小说.txt"})
	if err != nil {
		t.Fatal(err)
	}
	task = waitResourceTask(t, a, task.ID)
	if task.Status != "done" || task.BookName != "下载小说.txt" {
		t.Fatalf("%+v", task)
	}
	if _, err = os.Stat(filepath.Join(book.ParsePath(), task.BookName)); err != nil {
		t.Fatal(err)
	}
	originals, err := book.ListOriginals()
	if err != nil || len(originals) != 1 {
		t.Fatalf("%+v %v", originals, err)
	}
}

// TestCrawlerUsesSavedSourceForSearchAndDownload 验证自定义规则通过桌面入口参与搜索、详情解析和下载入库。
// 删除配置中的来源后，原来的搜索结果也无法再创建抓取任务。
func TestCrawlerUsesSavedSourceForSearchAndDownload(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/search":
			if req.Method != http.MethodPost || req.FormValue("title") != "测试小说" {
				t.Errorf("unexpected search: %s %v", req.Method, req.Form)
			}
			fmt.Fprint(w, `<div class="result"><a href="/book/42">测试小说</a><span class="author">搜索作者</span></div>`)
		case "/book/42":
			fmt.Fprint(w, `<meta name="description" content="测试简介"><h1>测试小说</h1><ol id="toc"><li><a href="/chapter/1">第一章 开始</a></li></ol>`)
		case "/chapter/1":
			fmt.Fprint(w, `<article>这是来自自定义书源的正文。</article>`)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	app := NewApp()
	defer app.stopResourceDownloads()
	config := fmt.Sprintf(`{"version":2,"sources":[{
		"id":"custom-crawler","name":"自定义书源","enabled":true,"kind":"web-crawler","group":"我的分组",
		"homeUrl":%q,"searchUrl":%q,"searchMethod":"POST","searchForm":{"title":"{query}"},
		"bookLinkPattern":"^/book/[0-9]+$","crawler":{
			"search":{"result":".result","bookName":"a","author":".author"},"toc":{"item":"#toc a"},"chapter":{"content":"article"}
		}
	}]}`, server.URL, server.URL+"/search")
	if err := app.SaveResourceSources(config); err != nil {
		t.Fatal(err)
	}
	results, err := app.FindDownloadableBooks("custom-crawler", "测试小说")
	if err != nil || len(results.Books) != 1 {
		t.Fatalf("search: %+v, %v", results, err)
	}
	resolved, err := app.ResolveResourceNovel(context.Background(), server.URL+"/book/42")
	if err != nil || resolved.SourceID != "custom-crawler" {
		t.Fatalf("resolve: %+v, %v", resolved, err)
	}
	file := results.Books[0].Files[0]
	file.Concurrency = 3
	task, err := app.StartResourceDownload(file)
	if err != nil {
		t.Fatal(err)
	}
	task = waitResourceTask(t, app, task.ID)
	if task.Status != "done" || task.Received != 1 || task.Total != 1 ||
		task.File.Concurrency != 3 || task.File.SourceGroup != "我的分组" ||
		task.Name != "测试小说.epub" || task.BookName != "测试小说.txt" {
		t.Fatalf("download: %+v", task)
	}
	if _, err := os.Stat(filepath.Join(book.PurePath(), "测试小说.epub")); err != nil {
		t.Fatal(err)
	}
	metadata, err := book.ListMetadata()
	if err != nil || len(metadata) != 1 || metadata[0].ImportType != "epub" || metadata[0].Title != "测试小说" ||
		len(metadata[0].Authors) != 1 || metadata[0].Authors[0] != "搜索作者" ||
		metadata[0].Description != "测试简介" || metadata[0].Cover == "" {
		t.Fatalf("bookshelf metadata: %+v, %v", metadata, err)
	}
	if err := app.SaveResourceSources(`{"version":2,"sources":[]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.StartResourceDownload(file); err == nil {
		t.Fatal("download accepted a removed source")
	}
	if _, err := app.ResolveResourceNovel(context.Background(), file.URL); err == nil {
		t.Fatal("link resolved using a removed source")
	}
}
func TestResourceDownloadCanCancelWithoutImporting(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	started := make(chan struct{})
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "第一章\n")
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	a := NewApp()
	defer a.stopResourceDownloads()
	task, err := a.StartResourceDownload(resourcecatalog.File{URL: server.URL, Name: "cancelled.txt"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request not started")
	}
	if err = a.CancelResourceDownload(task.ID); err != nil {
		t.Fatal(err)
	}
	task = waitResourceTask(t, a, task.ID)
	if task.Status != "cancelled" {
		t.Fatalf("%+v", task)
	}
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP request did not stop")
	}
	if _, err = os.Stat(filepath.Join(book.ParsePath(), "cancelled.txt")); !os.IsNotExist(err) {
		t.Fatal("cancelled resource entered library")
	}
}
func TestResourceDownloadDoesNotImportHTML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html>verification required</html>") }))
	defer server.Close()
	a := NewApp()
	defer a.stopResourceDownloads()
	task, err := a.StartResourceDownload(resourcecatalog.File{URL: server.URL, Name: "book.txt"})
	if err != nil {
		t.Fatal(err)
	}
	task = waitResourceTask(t, a, task.ID)
	if task.Status != "error" {
		t.Fatalf("%+v", task)
	}
	if _, err = os.Stat(filepath.Join(book.PurePath(), "book.txt")); !os.IsNotExist(err) {
		t.Fatal("HTML resource imported")
	}
}

func TestResourceDownloadZIPImportsExtractedBook(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	entry, _ := archive.Create("nested/original.txt")
	fmt.Fprint(entry, "第一章 开始\n小说正文\n")
	archive.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(body.Bytes())
	}))
	defer server.Close()
	a := NewApp()
	defer a.stopResourceDownloads()
	file := resourcecatalog.File{URL: server.URL, Name: "第一序列.zip", ArchiveFormat: "txt", Referer: server.URL + "/book"}
	task, err := a.StartResourceDownload(file)
	if err != nil {
		t.Fatal(err)
	}
	task = waitResourceTask(t, a, task.ID)
	if task.Status != "done" || task.BookName != "第一序列.txt" || task.File.ArchiveFormat != "txt" {
		t.Fatalf("%+v", task)
	}
	if _, err = os.Stat(filepath.Join(book.PurePath(), "第一序列.txt")); err != nil {
		t.Fatal(err)
	}
}
