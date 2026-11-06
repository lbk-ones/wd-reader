package book

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resourceLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := EnsureStorage(); err != nil {
		t.Fatal(err)
	}
	return resourcesPath()
}

func putResource(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func resourceContent(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func makeResourceBackup(t *testing.T) string {
	t.Helper()
	root := resourceLibrary(t)
	for name, content := range map[string]string{
		"parse/same.txt": "incoming text", "pure/same.txt": "incoming original",
		"parse/new.txt": "new text", "pure/new.txt": "new original",
		"meta-data/covers/cover.png": "incoming cover", "extra/data.bin": "extra file",
	} {
		putResource(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	if err := os.MkdirAll(filepath.Join(root, "extra", "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	store := metadataStore{Version: 1, CategoryPresetsAdded: true,
		Categories: []Category{{ID: "category-collision", Name: "Remote"}, {ID: "remote-shared", Name: "Shared"}},
		Books: []BookMetadata{
			{ID: "id-collision", FileName: "same.txt", OriginalName: "same.txt", Title: "Remote title", Favorite: true,
				CategoryID: "category-collision", Cover: "cover.png", CoverChecked: true},
			{ID: "new-id", FileName: "new.txt", OriginalName: "new.txt", Title: "New title", CategoryID: "remote-shared", CoverChecked: true},
		},
	}
	if err := writeMetadata(store); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "资源.zip")
	cache := FrontendCache{LocalStorage: map[string]string{"Wd-same.txt": "chapter 2", "custom": "all keys"}, SessionStorage: map[string]string{"session": "saved"}}
	if _, err := ExportResources(context.Background(), destination, cache, nil); err != nil {
		t.Fatal(err)
	}
	return destination
}

func TestResourcesRoundTripMergeAndRepeatedImport(t *testing.T) {
	archive := makeResourceBackup(t)
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	reader.Close()
	for _, name := range []string{resourceManifestName, resourceCacheName, "books/meta-data/metadata.json", "books/meta-data/metadata.json.bak", "books/extra/data.bin", "books/extra/empty/"} {
		if !entries[name] {
			t.Fatalf("export omitted %s", name)
		}
	}
	root := resourceLibrary(t)
	putResource(t, filepath.Join(root, "parse", "same.txt"), "local text")
	putResource(t, filepath.Join(root, "pure", "same.txt"), "local original")
	putResource(t, filepath.Join(root, "meta-data", "covers", "cover.png"), "local cover")
	putResource(t, filepath.Join(root, "local.bin"), "keep local extra")
	if err := writeMetadata(metadataStore{Version: 1, CategoryPresetsAdded: true,
		Categories: []Category{{ID: "category-collision", Name: "Local"}, {ID: "local-shared", Name: "Shared"}},
		Books:      []BookMetadata{{ID: "id-collision", FileName: "same.txt", OriginalName: "same.txt", Title: "Local title", Cover: "cover.png", CategoryID: "category-collision"}},
	}); err != nil {
		t.Fatal(err)
	}
	baseline := resourceContent(t, filepath.Join(metadataDir(), "metadata.json"))
	plan, err := PrepareResourceImport(context.Background(), archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if got := resourceContent(t, filepath.Join(metadataDir(), "metadata.json")); got != baseline {
		t.Fatal("preparing the import changed metadata")
	}
	if _, err := os.Stat(filepath.Join(ParsePath(), "new.txt")); !os.IsNotExist(err) {
		t.Fatal("preparing the import wrote live files")
	}
	if plan.BookNames["same.txt"] != "same (import 2).txt" || plan.Cache.LocalStorage["custom"] != "all keys" || plan.Cache.SessionStorage["session"] != "saved" {
		t.Fatalf("invalid preview: %+v %+v", plan.BookNames, plan.Cache)
	}
	if err := plan.Commit(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if plan.Stats.BooksAdded != 2 {
		t.Fatalf("book count: %+v", plan.Stats)
	}
	for name, content := range map[string]string{
		"parse/same.txt": "local text", "pure/same.txt": "local original", "local.bin": "keep local extra",
		"parse/same (import 2).txt": "incoming text", "pure/same (import 2).txt": "incoming original",
		"parse/new.txt": "new text", "extra/data.bin": "extra file",
		"meta-data/covers/cover.png": "local cover", "meta-data/covers/cover (import 2).png": "incoming cover",
	} {
		if got := resourceContent(t, filepath.Join(root, filepath.FromSlash(name))); got != content {
			t.Fatalf("%s = %q, want %q", name, got, content)
		}
	}
	if info, err := os.Stat(filepath.Join(root, "extra", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory was not preserved: %v", err)
	}
	store, err := readMetadata()
	if err != nil || len(store.Books) != 3 {
		t.Fatalf("merged metadata: %+v %v", store, err)
	}
	for _, book := range store.Books {
		switch book.FileName {
		case "same.txt":
			if book.Title != "Local title" || book.Favorite || book.CategoryID != "category-collision" {
				t.Fatalf("local metadata was overwritten: %+v", book)
			}
		case "same (import 2).txt":
			if book.ID == "id-collision" || !book.Favorite || book.Cover != "cover (import 2).png" || book.OriginalName != "same (import 2).txt" || book.CategoryID == "category-collision" {
				t.Fatalf("imported references were not remapped: %+v", book)
			}
		case "new.txt":
			if book.CategoryID != "local-shared" {
				t.Fatalf("same-named category was duplicated: %+v", book)
			}
		}
	}
	repeated, err := PrepareResourceImport(context.Background(), archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer repeated.Close()
	if repeated.Stats.FilesAdded != 0 || repeated.Stats.BooksAdded != 0 || repeated.BookNames["same.txt"] != plan.BookNames["same.txt"] {
		t.Fatalf("repeated import was not idempotent: %+v %+v", repeated.Stats, repeated.BookNames)
	}
	if err := repeated.Commit(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func writeTestResourceZIP(t *testing.T, entries map[string]string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "resources.zip")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	defaults := map[string]string{resourceManifestName: `{"format":"wd-reader-resources","version":1}`, resourceCacheName: `{"localStorage":{},"sessionStorage":{}}`}
	for name, content := range entries {
		defaults[name] = content
	}
	for name, content := range defaults {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestResourcesRejectMalformedArchivesBeforeChangingLibrary(t *testing.T) {
	for _, name := range []string{"../outside.txt", "books/../../outside.txt", "books/parse/../outside.txt", "books/parse\\outside.txt", "books/C:/outside.txt", "/books/outside.txt"} {
		t.Run(name, func(t *testing.T) {
			root := resourceLibrary(t)
			putResource(t, filepath.Join(root, "keep.txt"), "keep")
			archive := writeTestResourceZIP(t, map[string]string{name: "bad", "books/parse/new.txt": "text"})
			if plan, err := PrepareResourceImport(context.Background(), archive, nil); err == nil {
				plan.Close()
				t.Fatal("accepted unsafe archive path")
			}
			if got := resourceContent(t, filepath.Join(root, "keep.txt")); got != "keep" {
				t.Fatal("changed existing file")
			}
			if _, err := os.Stat(filepath.Join(ParsePath(), "new.txt")); !os.IsNotExist(err) {
				t.Fatal("partially imported invalid archive")
			}
		})
	}
	for _, entries := range []map[string]string{
		{resourceCacheName: `{"localStorage":{"setting":42}}`},
		{resourceManifestName: `{"format":"other","version":1}`},
		{"books/meta-data/metadata.json": `{"version":99,"books":[]}`},
		{"books/meta-data/metadata.json": `{"version":1,"books":[{"id":"a","fileName":"../../outside"}]}`},
		{"books/parse/a.txt": "a", "books/parse/A.txt": "b"},
	} {
		resourceLibrary(t)
		if plan, err := PrepareResourceImport(context.Background(), writeTestResourceZIP(t, entries), nil); err == nil {
			plan.Close()
			t.Fatalf("accepted malformed archive: %+v", entries)
		}
	}
}

func TestResourceCommitRollsBackFilesOnFailure(t *testing.T) {
	archive := makeResourceBackup(t)
	root := resourceLibrary(t)
	if err := writeMetadata(metadataStore{Version: 1, Books: []BookMetadata{}}); err != nil {
		t.Fatal(err)
	}
	baseline := resourceContent(t, filepath.Join(metadataDir(), "metadata.json"))
	plan, err := PrepareResourceImport(context.Background(), archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	// Another operation creates a destination after planning. Earlier new files
	// must be removed without touching the competing file or existing metadata.
	last := plan.copies[len(plan.copies)-1].destination
	putResource(t, filepath.Join(root, filepath.FromSlash(last)), "created meanwhile")
	if err := plan.Commit(context.Background(), nil); err == nil {
		t.Fatal("overwrote a destination created after planning")
	}
	if got := resourceContent(t, filepath.Join(root, filepath.FromSlash(last))); got != "created meanwhile" {
		t.Fatal("overwrote the competing file")
	}
	for _, file := range plan.copies[:len(plan.copies)-1] {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.destination))); !os.IsNotExist(err) {
			t.Fatalf("left partially imported file %s", file.destination)
		}
	}
	if resourceContent(t, filepath.Join(metadataDir(), "metadata.json")) != baseline {
		t.Fatal("changed metadata on failure")
	}
}

func TestResourcePlanRejectsChangedMetadataAndPreservesExportDestination(t *testing.T) {
	archive := makeResourceBackup(t)
	resourceLibrary(t)
	plan, err := PrepareResourceImport(context.Background(), archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err := writeMetadata(metadataStore{Version: 1, Categories: []Category{{ID: "new", Name: "Changed"}}}); err != nil {
		t.Fatal(err)
	}
	if err := plan.Commit(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "书库已变化") {
		t.Fatalf("accepted a stale merge plan: %v", err)
	}
	// An interrupted export must keep the previous archive intact.
	destination := filepath.Join(t.TempDir(), "backup.zip")
	putResource(t, destination, "existing backup")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExportResources(ctx, destination, FrontendCache{}, nil); err == nil {
		t.Fatal("accepted cancelled export")
	}
	if resourceContent(t, destination) != "existing backup" {
		t.Fatal("cancelled export destroyed the previous archive")
	}
	inside := filepath.Join(resourcesPath(), "backup.zip")
	if _, err := ExportResources(context.Background(), inside, FrontendCache{}, nil); err == nil {
		t.Fatal("allowed exporting an archive inside its own source tree")
	}
}

func TestResourceImportedMetadataRejectsDuplicateIDs(t *testing.T) {
	data, _ := json.Marshal(metadataStore{Version: 1, Books: []BookMetadata{{ID: "same", FileName: "a.txt"}, {ID: "same", FileName: "b.txt"}}})
	if _, err := decodeResourceMetadata(data); err == nil {
		t.Fatal("accepted duplicate book identifiers")
	}
}

func TestResourceRenamingDoesNotConsumeAnotherIncomingFilename(t *testing.T) {
	root := resourceLibrary(t)
	putResource(t, filepath.Join(root, "parse", "a.txt"), "local")
	archive := writeTestResourceZIP(t, map[string]string{
		"books/parse/a.txt": "first incoming book", "books/parse/a (import 2).txt": "second incoming book",
	})
	plan, err := PrepareResourceImport(context.Background(), archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if plan.BookNames["a.txt"] == "a (import 2).txt" || plan.BookNames["a (import 2).txt"] != "a (import 2).txt" {
		t.Fatalf("consumed another incoming filename: %+v", plan.BookNames)
	}
	if err := plan.Commit(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if resourceContent(t, filepath.Join(ParsePath(), plan.BookNames["a.txt"])) != "first incoming book" ||
		resourceContent(t, filepath.Join(ParsePath(), "a (import 2).txt")) != "second incoming book" {
		t.Fatal("lost one of the incoming books")
	}
}

func TestResourceImportPreservesMissingBookRecords(t *testing.T) {
	resourceLibrary(t)
	store := metadataStore{Version: 1, Books: []BookMetadata{{ID: "missing", FileName: "missing.txt", Title: "Missing"}}}
	data, _ := json.Marshal(store)
	archive := writeTestResourceZIP(t, map[string]string{"books/meta-data/metadata.json": string(data)})
	for range 2 {
		plan, err := PrepareResourceImport(context.Background(), archive, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Commit(context.Background(), nil); err != nil {
			plan.Close()
			t.Fatal(err)
		}
		plan.Close()
	}
	current, err := readMetadata()
	if err != nil || len(current.Books) != 1 || current.Books[0].FileName != "missing.txt" {
		t.Fatalf("duplicated missing-file records: %+v %v", current, err)
	}
}
