package book

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMetadataLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(GetAppPath(), "books"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(GetAppPath(), "books", "a.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}
	list, err := ListMetadata()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	id := list[0].ID
	if list[0].ImportType != "txt" {
		t.Fatalf("discovered file type: %q", list[0].ImportType)
	}
	imported := list[0].ImportedAt
	if imported.IsZero() {
		t.Fatal("missing import time")
	}
	if err = UpdateMetadata(id, "书名", []string{"作者"}, "简介"); err != nil {
		t.Fatal(err)
	}
	imageData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nmWQAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	coverFile := filepath.Join(dir, "cover.png")
	if err = os.WriteFile(coverFile, imageData, 0644); err != nil {
		t.Fatal(err)
	}
	if err = SetCover(id, coverFile); err != nil {
		t.Fatal(err)
	}
	cover, err := CoverData(id)
	if err != nil || cover == "" {
		t.Fatalf("cover: %q %v", cover, err)
	}
	if err = CaptureMOBIMetadata("a.txt", "other", []string{"other"}, "other"); err != nil {
		t.Fatal(err)
	}
	list, err = ListMetadata()
	if err != nil || list[0].ID != id || list[0].Title != "书名" || list[0].Authors[0] != "作者" {
		t.Fatalf("metadata: %#v %v", list, err)
	}
	if err = os.Rename(filepath.Join(GetAppPath(), "books", "a.txt"), filepath.Join(GetAppPath(), "books", "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	list, err = ListMetadata()
	if err != nil || list[0].ID != id || list[0].FileName != "renamed.txt" || list[0].ImportType != "txt" || !list[0].ImportedAt.Equal(imported) {
		t.Fatalf("rename: %#v %v", list, err)
	}
	if err = RemoveMetadata("renamed.txt"); err != nil {
		t.Fatal(err)
	}
	list, err = ListMetadata()
	if err != nil || list[0].ID == id {
		t.Fatalf("removal: %#v %v", list, err)
	}
}

func TestLegacyImportTimeMigration(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	books := filepath.Join(GetAppPath(), "books")
	if err := os.MkdirAll(books, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(books, "old.txt")
	if err := os.WriteFile(file, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if err := writeMetadata(metadataStore{Version: 1, Books: []BookMetadata{{ID: "old", FileName: "old.txt", Title: "old"}}}); err != nil {
		t.Fatal(err)
	}
	list, err := ListMetadata()
	if err != nil || len(list) != 1 || !list[0].ImportedAt.Equal(old) {
		t.Fatalf("migration: %#v %v", list, err)
	}
	if list[0].ImportType != "" {
		t.Fatalf("legacy format must remain unknown: %q", list[0].ImportType)
	}
	if err = RecordImport("old.txt", "old.txt"); err != nil {
		t.Fatal(err)
	}
	list, err = ListMetadata()
	if err != nil || !list[0].ImportedAt.After(old) {
		t.Fatalf("reimport: %#v %v", list, err)
	}
}

func TestCaptureEPUBMetadata(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(GetAppPath(), "books"), 0755); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, "book.epub")
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, content := range map[string]string{"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OEBPS/book.opf"/></rootfiles></container>`, "OEBPS/book.opf": `<package xmlns:dc="http://purl.org/dc/elements/1.1/"><metadata><dc:title>原书名</dc:title><dc:creator>原作者</dc:creator><dc:description>简介</dc:description></metadata><manifest/></package>`} {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(content)); e != nil {
			t.Fatal(e)
		}
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = CaptureEPUBMetadata(filename, "book.txt"); err != nil {
		t.Fatal(err)
	}
	s, err := readMetadata()
	if err != nil || len(s.Books) != 1 || s.Books[0].Title != "原书名" || s.Books[0].Authors[0] != "原作者" || s.Books[0].ImportType != "epub" {
		t.Fatalf("metadata: %#v %v", s, err)
	}
}

func TestRecordOriginalImportType(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	for _, original := range []string{"book.txt", "book.EPUB", "book.mobi", "book.azw", "book.azw3", "book.pdf"} {
		if err := RecordImport("book.txt", original); err != nil {
			t.Fatal(err)
		}
		store, err := readMetadata()
		if err != nil {
			t.Fatal(err)
		}
		want := importType(original)
		if len(store.Books) != 1 || store.Books[0].ImportType != want {
			t.Fatalf("%s: got %#v, want %s", original, store.Books, want)
		}
	}
}

func TestMetadataDirectoryMigration(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	old := filepath.Join(GetAppPath(), "book-data")
	if err := os.MkdirAll(filepath.Join(old, "covers"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "metadata.json"), []byte(`{"version":1,"books":[{"id":"book","fileName":"book.txt"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "covers", "cover.png"), []byte("cover"), 0644); err != nil {
		t.Fatal(err)
	}
	store, err := readMetadata()
	if err != nil || len(store.Books) != 1 || store.Books[0].ID != "book" {
		t.Fatalf("migration: %#v %v", store, err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old directory remains: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(metadataDir(), "covers", "cover.png")); err != nil || string(data) != "cover" {
		t.Fatalf("cover: %q %v", data, err)
	}
}

func TestMetadataDirectoryMigrationDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	old := filepath.Join(GetAppPath(), "book-data")
	if err := os.MkdirAll(old, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "metadata.json"), []byte(`{"version":1,"books":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(metadataDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := readMetadata(); err == nil {
		t.Fatal("expected conflict for incomplete destination")
	}
	if _, err := os.Stat(filepath.Join(old, "metadata.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadataDir(), "metadata.json"), []byte(`{"version":1,"books":[{"id":"new"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	store, err := readMetadata()
	if err != nil || len(store.Books) != 1 || store.Books[0].ID != "new" {
		t.Fatalf("destination: %#v %v", store, err)
	}
}
