package desktop

import (
	"archive/zip"
	"bytes"
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
