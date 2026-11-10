package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
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
	path,
	title,
	author,
	category string
	key,
	packagePath,
	channel string
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
	return readBookEdition(filename, category, false)
}

// 已发布的输出可使用写回时记录的原文件指纹，继续按原先的规则识别缺少作者的版本。
func readBookEdition(filename, category string, published bool) (edition, error) {
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
	book.packagePath = container.Packages[0].Path
	var pkg struct {
		Title   string   `xml:"metadata>title"`
		Authors []string `xml:"metadata>creator"`
		Types   []string `xml:"metadata>type"`
		Meta    []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"metadata>meta"`
	}
	if err := readXML(archive, book.packagePath, &pkg); err != nil {
		return book, err
	}
	for _, kind := range pkg.Types {
		kind = normalizeChannel(kind)
		if kind == maleChannel || kind == femaleChannel {
			book.channel = kind
			break
		}
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
	if published {
		for _, meta := range pkg.Meta {
			if meta.Name == sourceDigestMeta {
				if digest, err := hex.DecodeString(meta.Content); err == nil && len(digest) == sha256.Size {
					book.key = fmt.Sprintf("file:%x", digest)
					return book, nil
				}
			}
		}
	}
	digest, err := fileDigest(filename)
	if err != nil {
		return book, err
	}
	book.key = fmt.Sprintf("file:%x", digest)
	return book, nil
}

func readXML(archive *zip.ReadCloser, name string, target any) error {
	data, err := readMetadataEntry(archive, name)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func readMetadataEntry(archive *zip.ReadCloser, name string) ([]byte, error) {
	const limit = 4 << 20
	f, err := archive.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s 超出元数据大小限制", name)
	}
	return data, nil
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
