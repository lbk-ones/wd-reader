package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"wd-reader/internal/epubexport"
)

func TestMergeBooksSelectsEditionAndPreservesDifferentAuthors(t *testing.T) {
	root := t.TempDir()
	create := func(group, category, filename, title, author, body string) string {
		t.Helper()
		dir := filepath.Join(root, group, category)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dir, filename+".epub")
		b := epubexport.Book{Title: title, Author: author}
		cover := []byte("\x89PNG\r\n\x1a\n")
		if err := epubexport.Write(target, b, []string{"第一章"}, cover, func(int) ([]string, error) {
			return []string{body}, nil
		}); err != nil {
			t.Fatal(err)
		}
		return target
	}
	create("book-a", "都市小说", "短版 [123]", "[娱乐圈]同名书 [123]", "甲", "少量正文。")
	best := create("books-b", "未分类", "最佳版 [4b5a56e6d8d7]（正文不全）", "[娱乐圈]同名书（正文不全） [4b5a56e6d8d7]", "甲", "这是正文更完整的版本。第二段也存在。")
	other := create("books-b", "都市青春", "另一个作者", "[娱乐圈]同名书", "乙", "其他作者的书。")

	if err := run(root, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "all")); !os.IsNotExist(err) {
		t.Fatalf("preview modified output: %v", err)
	}
	for range 2 {
		if err := run(root, true, false); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := filepath.Glob(filepath.Join(root, "all", "都市", "*.epub"))
	if err != nil || len(paths) != 2 {
		t.Fatalf("output paths = %v, %v", paths, err)
	}
	for i, want := range []string{"[娱乐圈]同名书 - 乙.epub", "[娱乐圈]同名书 - 甲.epub"} {
		if filepath.Base(paths[i]) != want {
			t.Fatalf("got name %q, want %q", filepath.Base(paths[i]), want)
		}
	}
	digestOf := func(path string) [32]byte {
		t.Helper()
		digest, err := fileDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	got := [][32]byte{digestOf(paths[0]), digestOf(paths[1])}
	for _, path := range []string{best, other} {
		if !slices.Contains(got, digestOf(path)) {
			t.Fatalf("missing chosen edition %s", path)
		}
	}
}

func TestCleanBookNameOnlyStripsSuffixes(t *testing.T) {
	for input, want := range map[string]string{
		"[娱乐圈]我在剧组 [12345]（正文不全）": "[娱乐圈]我在剧组",
		"回家（正文不全） ［12ab34cd56ef］": "回家",
		"名著 [9] 续篇(正文不全)":         "名著 [9] 续篇",
		"冒险 [番外]":                 "冒险 [番外]",
	} {
		if got := cleanBookName(input); got != want {
			t.Errorf("cleanBookName(%q) = %q, want %q", input, got, want)
		}
	}
}
