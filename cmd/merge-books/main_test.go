package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"wd-reader/internal/epubexport"
)

func TestMergeBooksSelectsEditionAndPreservesDifferentAuthors(t *testing.T) {
	root := t.TempDir()
	create := func(group, category, filename, title, author, body string) string {
		t.Helper()
		target := filepath.Join(root, group, category, filename+".epub")
		writeTestEdition(t, target, epubexport.Book{Title: title, Author: author, Category: category}, body)
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
	assertMergedEdition(t, paths[0], other, "都市", unknownChannel)
	assertMergedEdition(t, paths[1], best, "都市", unknownChannel)
}

func TestMergeBooksUpdatesOldCopyWithoutAuthor(t *testing.T) {
	root := t.TempDir()
	title := "开局签到十万年"
	source := filepath.Join(root, "book-a", "玄幻小说", title+".epub")
	writeTestEdition(t, source, epubexport.Book{Title: title, Category: "玄幻小说"}, "正文。")
	original := readTestFile(t, source)
	target := filepath.Join(root, "all", "玄幻奇幻", title+".epub")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(root, false, false); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readTestFile(t, target), original) {
		t.Fatal("preview changed the old copy")
	}
	for range 2 {
		if err := run(root, true, false); err != nil {
			t.Fatal(err)
		}
	}
	assertMergedEdition(t, target, source, "玄幻奇幻", maleChannel)
	if !bytes.Equal(readTestFile(t, source), original) {
		t.Fatal("source EPUB changed")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(target), "*.epub"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("output paths = %v, %v", paths, err)
	}
}

func TestMergeBooksKeepsConflictingCopy(t *testing.T) {
	root := t.TempDir()
	book := epubexport.Book{Title: "真千金今天也在和离", Author: "作者", Category: "现代言情"}
	source := filepath.Join(root, "book-a", "现代言情", "source.epub")
	target := filepath.Join(root, "all", "现言", "existing.epub")
	writeTestEdition(t, source, book, "入选正文。")
	writeTestEdition(t, target, book, "不同的正文。")
	existing := readTestFile(t, target)
	if err := run(root, true, false); err == nil {
		t.Fatal("merge accepted a different edition")
	}
	if !bytes.Equal(readTestFile(t, target), existing) {
		t.Fatal("conflicting copy changed")
	}
}

func TestInferChannel(t *testing.T) {
	for _, tc := range []struct{ title, category, old, want string }{
		{"《开局签到，我在诸天无敌》", "玄幻奇幻", "", maleChannel},
		{"【快穿】恶毒女配今天团宠了吗", "未分类", "", femaleChannel},
		{"［女頻］神豪的退休生活", "都市", "", femaleChannel},
		{"穿书后我成了男频文的女配", "未分类", "", femaleChannel},
		{"重生后我穿越了", "穿越", "", unknownChannel},
		{"神豪的白月光", "都市", "", unknownChannel},
		{"春风又绿", "现言", "", femaleChannel},
		{"长夜", "科幻", maleChannel, maleChannel},
	} {
		if got := inferChannel(tc.title, tc.category, tc.old); got != tc.want {
			t.Errorf("inferChannel(%q, %q, %q) = %q, want %q", tc.title, tc.category, tc.old, got, tc.want)
		}
	}
}

func TestRewritePackagePreservesUnrelatedMetadata(t *testing.T) {
	data := []byte(`<opf:package xmlns:opf="http://www.idpf.org/2007/opf" xmlns:d="http://purl.org/dc/elements/1.1/">
  <opf:metadata>
    <d:title id="title">书名</d:title>
    <d:subject id="old">旧分类</d:subject><d:subject>第二个分类</d:subject>
    <opf:meta refines="#scheme" property="test:comment">旧分类说明</opf:meta>
    <opf:meta refines="#old" property="authority" id="scheme">旧分类体系</opf:meta>
    <opf:meta refines="#title" property="title-type">main</opf:meta>
    <d:type>Text</d:type><d:type>男頻</d:type>
  </opf:metadata><opf:manifest/><opf:spine/>
</opf:package>`)
	updated, err := rewritePackage(data, "现言 & 都市", femaleChannel, "")
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Subjects []string `xml:"metadata>subject"`
		Types    []string `xml:"metadata>type"`
	}
	if err := xml.Unmarshal(updated, &pkg); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pkg.Subjects, []string{"现言 & 都市"}) || !slices.Equal(pkg.Types, []string{"Text", femaleChannel}) {
		t.Fatalf("metadata = %+v", pkg)
	}
	for _, want := range []string{`<opf:meta refines="#title" property="title-type">main</opf:meta>`, `<opf:manifest/><opf:spine/>`} {
		if !strings.Contains(string(updated), want) {
			t.Errorf("lost %q", want)
		}
	}
	for _, obsolete := range []string{`refines="#old"`, `refines="#scheme"`} {
		if strings.Contains(string(updated), obsolete) {
			t.Errorf("kept stale reference %q", obsolete)
		}
	}
}

func writeTestEdition(t *testing.T, target string, book epubexport.Book, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	cover := []byte("\x89PNG\r\n\x1a\n")
	if err := epubexport.Write(target, book, []string{"第一章"}, cover, func(int) ([]string, error) {
		return []string{body}, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertMergedEdition(t *testing.T, target, source, category, channel string) {
	t.Helper()
	open := func(name string) *zip.ReadCloser {
		t.Helper()
		archive, err := zip.OpenReader(name)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { archive.Close() })
		return archive
	}
	got, original := open(target), open(source)
	const packagePath = "EPUB/package.opf"
	var pkg struct {
		Subjects []string `xml:"metadata>subject"`
		Types    []string `xml:"metadata>type"`
	}
	if err := readXML(got, packagePath, &pkg); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pkg.Subjects, []string{category}) || !slices.Equal(pkg.Types, []string{channel}) {
		t.Fatalf("metadata = %+v, want subject %q and type %q", pkg, category, channel)
	}
	if len(got.File) != len(original.File) || got.File[0].Name != "mimetype" || got.File[0].Method != zip.Store {
		t.Fatal("output EPUB has a different ZIP layout")
	}
	raw := func(file *zip.File) []byte {
		t.Helper()
		r, err := file.OpenRaw()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for i, file := range original.File {
		if entry := got.File[i]; entry.Name != file.Name || (file.Name != packagePath && !bytes.Equal(raw(entry), raw(file))) {
			t.Errorf("ZIP entry %q changed", file.Name)
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
