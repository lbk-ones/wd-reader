package epubexport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode"
)

const epubType = "application/epub+zip"

var epubTemplates = template.Must(template.New("epub").Funcs(template.FuncMap{
	"xml": func(s string) string {
		var out bytes.Buffer
		xml.EscapeText(&out, []byte(s))
		return out.String()
	},
	"chapterID": func(i int) string { return fmt.Sprintf("chapter-%04d", i+1) },
	"page": func(title, language string) map[string]string {
		return map[string]string{"Title": title, "Language": language}
	},
}).Parse(`
{{define "package.opf"}}<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:identifier id="book-id">{{xml .Source}}</dc:identifier>
  <dc:title>{{xml .Title}}</dc:title>
  {{if .Author}}<dc:creator>{{xml .Author}}</dc:creator>{{end}}
  <dc:language>{{xml .Language}}</dc:language>
  {{if .Category}}<dc:subject>{{xml .Category}}</dc:subject>{{end}}
  {{if .Description}}<dc:description>{{xml .Description}}</dc:description>{{end}}
  <dc:source>{{xml .Source}}</dc:source>
  <meta property="dcterms:modified">{{.Modified}}</meta>
  <meta name="cover" content="cover"/>
</metadata>
<manifest>
  <item id="cover" href="{{.Cover}}" media-type="{{.CoverType}}" properties="cover-image"/>
  <item id="intro" href="intro.xhtml" media-type="application/xhtml+xml"/>
  <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  {{range $i, $_ := .Chapters}}<item id="{{chapterID $i}}" href="{{chapterID $i}}.xhtml" media-type="application/xhtml+xml"/>
  {{end}}
</manifest>
<spine>
  <itemref idref="intro"/>
  <itemref idref="nav"/>
  {{range $i, $_ := .Chapters}}<itemref idref="{{chapterID $i}}"/>
  {{end}}
</spine>
</package>{{end}}
{{define "start"}}<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="{{xml .Language}}" lang="{{xml .Language}}">
<head><title>{{xml .Title}}</title></head><body>{{end}}
{{define "end"}}</body></html>{{end}}
{{define "intro.xhtml"}}{{template "start" .}}
  <h1>{{xml .Title}}</h1>
  <p><img src="{{.Cover}}" alt="{{xml .Title}}封面" style="max-width: 100%; max-height: 70vh;"/></p>
  {{if .Author}}<p>作者：{{xml .Author}}</p>{{end}}
  {{if .Category}}<p>分类：{{xml .Category}}</p>{{end}}
  {{if .Intro}}<h2>简介</h2>{{end}}
  {{range .Intro}}<p>{{xml .}}</p>
  {{end}}
{{template "end"}}{{end}}
{{define "nav.xhtml"}}{{template "start" (page "目录" .Language)}}
  <nav epub:type="toc" id="toc"><h1>目录</h1><ol>
  <li><a href="intro.xhtml">封面与简介</a></li>
  {{range $i, $ch := .Chapters}}<li><a href="{{chapterID $i}}.xhtml">{{xml $ch}}</a></li>
  {{end}}</ol></nav>
{{template "end"}}{{end}}
{{define "chapter"}}{{template "start" .}}
  <h1>{{xml .Title}}</h1>
  {{range .Paragraphs}}<p>{{xml .}}</p>
  {{end}}
{{template "end"}}{{end}}
`))

const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

type Book struct {
	Title, Author, Category, Source string
	Description                     []string
	// Language 保存 EPUB 语言标记。留空时使用 zh-CN。
	Language string
}

func Write(target string, b Book, titles []string, cover []byte, body func(int) ([]string, error)) error {
	coverFile, coverType := CoverFormat(cover)
	if coverFile == "" {
		return fmt.Errorf("无法识别封面图片: %s", coverType)
	}
	if b.Language == "" {
		b.Language = "zh-CN"
	}
	info := struct {
		Title, Author, Category, Source, Modified, Description, Cover, CoverType, Language string
		Intro, Chapters                                                                    []string
	}{
		Title:       b.Title,
		Author:      b.Author,
		Category:    b.Category,
		Source:      b.Source,
		Modified:    time.Now().UTC().Format(time.RFC3339),
		Description: strings.Join(b.Description, "\n"),
		Intro:       b.Description,
		Cover:       coverFile,
		CoverType:   coverType,
		Chapters:    titles,
		Language:    b.Language,
	}

	f, err := os.CreateTemp(filepath.Dir(target), ".epub-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	archive := zip.NewWriter(f)
	mimeEntry, err := archive.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(mimeEntry, epubType); err != nil {
		return err
	}
	containerEntry, err := archive.Create("META-INF/container.xml")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(containerEntry, containerXML); err != nil {
		return err
	}
	image, err := archive.Create("EPUB/" + coverFile)
	if err != nil {
		return err
	}
	if _, err := image.Write(cover); err != nil {
		return err
	}
	for _, name := range []string{"package.opf", "intro.xhtml", "nav.xhtml"} {
		if err := writeDocument(archive, name, name, info); err != nil {
			return err
		}
	}
	for i, title := range titles {
		paragraphs, err := body(i)
		if err != nil {
			return fmt.Errorf("第 %d/%d 章《%s》: %w", i+1, len(titles), title, err)
		}
		content := struct {
			Title, Language string
			Paragraphs      []string
		}{Title: title, Language: b.Language, Paragraphs: paragraphs}
		if err := writeDocument(archive, fmt.Sprintf("chapter-%04d.xhtml", i+1), "chapter", content); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// 发布已完成的文件，且同名输出已存在时不覆盖。
	return os.Link(f.Name(), target)
}

func writeDocument(archive *zip.Writer, name, tpl string, data any) error {
	w, err := archive.Create("EPUB/" + name)
	if err != nil {
		return err
	}
	return epubTemplates.ExecuteTemplate(w, tpl, data)
}

func SafeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	runes := []rune(name)
	name = strings.Trim(string(runes[:min(len(runes), 80)]), " .")
	if name == "" || !filepath.IsLocal(name) {
		return "_" + name
	}
	return name
}
