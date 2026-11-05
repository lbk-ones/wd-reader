package book

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryImportDeleteAndRestore(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "book.txt")
	if err := os.WriteFile(source, []byte("第一章\n正文"), 0644); err != nil {
		t.Fatal(err)
	}
	name, err := ImportLocal(source)
	if err != nil || name != "book.txt" {
		t.Fatalf("import %q %v", name, err)
	}
	list, err := ListMetadata()
	if err != nil || len(list) != 1 {
		t.Fatalf("metadata %#v %v", list, err)
	}
	if list[0].OriginalName != "book.txt" {
		t.Fatalf("original association: %#v", list[0])
	}
	if err = UpdateMetadata(list[0].ID, "edited", nil, ""); err != nil {
		t.Fatal(err)
	}
	originals, err := ListOriginals()
	if err != nil || len(originals) != 1 || !originals[0].InShelf {
		t.Fatalf("originals %#v %v", originals, err)
	}
	if result := DeleteFile(name); result != "" {
		t.Fatal(result)
	}
	if err = RemoveMetadata(name); err != nil {
		t.Fatal(err)
	}
	originals, err = ListOriginals()
	if err != nil || originals[0].InShelf {
		t.Fatalf("deleted shelf %#v %v", originals, err)
	}
	if _, err = ImportOriginal(originals[0].Name, false); err != nil {
		t.Fatal(err)
	}
	list, err = ListMetadata()
	if err != nil || len(list) != 1 || list[0].Title == "edited" {
		t.Fatalf("restored metadata %#v %v", list, err)
	}
	if list[0].OriginalName != "book.txt" {
		t.Fatalf("restored association: %#v", list[0])
	}
	if _, err = ImportOriginal(originals[0].Name, false); err == nil {
		t.Fatal("existing shelf copy should require confirmation")
	}
	if _, err = ImportOriginal(originals[0].Name, true); err != nil {
		t.Fatal(err)
	}
	if err = DeleteOriginal(originals[0].Name); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(ParsePath(), name)); err != nil {
		t.Fatalf("shelf copy removed: %v", err)
	}
}

func TestImportNamesAndFailedParseKeepOriginal(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	input := t.TempDir()
	source := filepath.Join(input, "same.txt")
	if err := os.WriteFile(source, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLocal(source); err != nil {
		t.Fatal(err)
	}
	second, err := ImportLocal(source)
	if err != nil || second != "same (2).txt" {
		t.Fatalf("second %q %v", second, err)
	}
	broken := filepath.Join(input, "same.epub")
	if err = os.WriteFile(broken, []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = ImportLocal(broken); err == nil {
		t.Fatal("expected invalid epub error")
	}
	originals, err := ListOriginals()
	if err != nil || len(originals) != 3 {
		t.Fatalf("originals %#v %v", originals, err)
	}
	found := false
	for _, item := range originals {
		if strings.HasSuffix(item.Name, ".epub") {
			found = true
			if item.InShelf {
				t.Fatal("failed import reported on shelf")
			}
		}
	}
	if !found {
		t.Fatal("failed original missing")
	}
	if err = DeleteOriginal("../book.txt"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestLegacyShelfMigrationAndConflict(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	root := filepath.Join(GetAppPath(), "books")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "old.txt")
	if err := os.WriteFile(legacy, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureStorage(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(PurePath(), "old.txt"), filepath.Join(ParsePath(), "old.txt")} {
		if b, err := os.ReadFile(p); err != nil || string(b) != "old" {
			t.Fatalf("%s %q %v", p, b, err)
		}
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy remains %v", err)
	}
	if err := os.WriteFile(legacy, []byte("different"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureStorage(); err == nil {
		t.Fatal("conflict should fail")
	}
	if b, err := os.ReadFile(legacy); err != nil || string(b) != "different" {
		t.Fatalf("legacy damaged %q %v", b, err)
	}
}

func TestFailedRestoreKeepsShelfCopy(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := EnsureStorage(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(PurePath(), "book.epub"), []byte("bad epub"), 0644); err != nil {
		t.Fatal(err)
	}
	parsed := filepath.Join(ParsePath(), "book.txt")
	if err := os.WriteFile(parsed, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportOriginal("book.epub", true); err == nil {
		t.Fatal("expected parse failure")
	}
	if data, err := os.ReadFile(parsed); err != nil || string(data) != "existing" {
		t.Fatalf("shelf damaged: %q %v", data, err)
	}
}
