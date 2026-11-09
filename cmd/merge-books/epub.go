package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	epubsource "wd-reader/internal/chapterdetect/source"
	"wd-reader/internal/epubexport"
)

type edition struct {
	path, title, author, category string
	key                           string
}

var sourceIDSuffix = regexp.MustCompile(`(?:\[(?:[0-9]+|[0-9a-fA-F]{12}|[0-9a-fA-F]{16})\]|［(?:[0-9]+|[0-9a-fA-F]{12}|[0-9a-fA-F]{16})］)$`)

func cleanBookName(name string) string {
	name = strings.TrimSpace(name)
	for {
		before := name
		name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(name, "（正文不全）"), "(正文不全)"))
		if id := sourceIDSuffix.FindStringIndex(name); id != nil {
			name = strings.TrimSpace(name[:id[0]])
		}
		if name == before {
			return name
		}
	}
}

func readEdition(filename, category string) (edition, error) {
	book := edition{path: filename, category: category}
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return book, err
	}
	defer archive.Close()
	var container struct {
		Packages []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := readXML(archive, "META-INF/container.xml", &container); err != nil {
		return book, err
	}
	if len(container.Packages) == 0 {
		return book, fmt.Errorf("EPUB 缺少 OPF")
	}
	var pkg struct {
		Title   string   `xml:"metadata>title"`
		Authors []string `xml:"metadata>creator"`
	}
	if err := readXML(archive, container.Packages[0].Path, &pkg); err != nil {
		return book, err
	}
	book.title = cleanBookName(epubexport.UnescapeHTML(pkg.Title))
	var authors, authorKeys []string
	unknownAuthor := false
	for _, author := range pkg.Authors {
		author = strings.TrimSpace(epubexport.UnescapeHTML(author))
		key := normalize(author)
		switch key {
		case "", "未知", "未知作者", "作者未知", "不详", "佚名", "无名氏", "anonymous", "unknown", "unknown author":
			unknownAuthor = true
			continue
		}
		if !slices.Contains(authorKeys, key) {
			authors = append(authors, author)
			authorKeys = append(authorKeys, key)
		}
	}
	book.author = strings.Join(authors, "、")
	titleKey := normalize(book.title)
	if strings.HasPrefix(titleKey, "《") && strings.HasSuffix(titleKey, "》") {
		titleKey = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(titleKey, "《"), "》"))
	}
	if titleKey == "" {
		book.title = cleanBookName(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	}
	if titleKey != "" && len(authorKeys) > 0 && !unknownAuthor {
		slices.Sort(authorKeys)
		book.key = "metadata\x00" + titleKey + "\x00" + strings.Join(authorKeys, "\x00")
		return book, nil
	}
	digest, err := fileDigest(filename)
	if err != nil {
		return book, err
	}
	book.key = fmt.Sprintf("file:%x", digest)
	return book, nil
}

func readXML(archive *zip.ReadCloser, name string, target any) error {
	const limit = 4 << 20
	f, err := archive.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if len(data) > limit {
		return fmt.Errorf("%s 超出元数据大小限制", name)
	}
	if err := xml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func normalize(text string) string {
	return cases.Fold().String(strings.Join(strings.Fields(norm.NFKC.String(text)), " "))
}

func fileDigest(path string) ([sha256.Size]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	return [sha256.Size]byte(hash.Sum(nil)), nil
}

type quality struct {
	text     int
	chapters int
}

func assess(book edition) (quality, error) {
	doc, err := epubsource.ReadEPUB(book.path)
	if err != nil {
		return quality{}, err
	}
	q := quality{chapters: len(doc.Native)}
	for _, line := range doc.Lines {
		q.text += utf8.RuneCountInString(strings.TrimSpace(line))
	}
	return q, nil
}

func (q quality) betterThan(other quality) bool {
	if q.text != other.text {
		return q.text > other.text
	}
	return q.chapters > other.chapters
}
