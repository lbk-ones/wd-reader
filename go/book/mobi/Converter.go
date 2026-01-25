package mobi

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
)

// ToEpub 将 Mobi 文件转换为 EPUB 格式
// outputPath: 输出 EPUB 文件的路径
func (r *Reader) ToEpub(outputPath string) error {
	// 1. 获取内容
	htmlContent, err := r.ExtractHtml()
	if err != nil {
		return fmt.Errorf("failed to extract html: %w", err)
	}

	// 2. 准备元数据
	title := "Unknown Title"
	if len(r.MobiBook.Headers.MobiHeader.Title) > 0 {
		title = string(r.MobiBook.Headers.MobiHeader.Title)
	}
	language := "en"
	if r.MobiBook.Headers.MobiHeader.Language != "" {
		language = r.MobiBook.Headers.MobiHeader.Language
	}
	uid := uuid.New().String()

	// 3. 创建 ZIP 文件
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// 4. 写入 mimetype (必须是第一个文件，且不压缩)
	mimeHeader := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store, // 不压缩
	}
	w, err := zw.CreateHeader(mimeHeader)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("application/epub+zip"))
	if err != nil {
		return err
	}

	// 5. 写入 META-INF/container.xml
	err = writeStringFile(zw, "META-INF/container.xml", containerXml)
	if err != nil {
		return err
	}

	// 6. 写入 OEBPS/content.opf
	opfData := OpfData{
		Title:      title,
		Language:   language,
		Identifier: uid,
		Date:       time.Now().Format("2006-01-02"),
	}
	err = writeTemplateFile(zw, "OEBPS/content.opf", opfTemplate, opfData)
	if err != nil {
		return err
	}

	// 7. 写入 OEBPS/toc.ncx
	ncxData := NcxData{
		Title:      title,
		Identifier: uid,
	}
	err = writeTemplateFile(zw, "OEBPS/toc.ncx", ncxTemplate, ncxData)
	if err != nil {
		return err
	}

	// 8. 写入内容文件 OEBPS/Text/content.xhtml
	// 需要将 Mobi 的 HTML 包装成 XHTML
	// Mobi HTML 可能是片段，也可能包含 <html> 标签。
	// 这里做一个简单的包装。
	finalHtml := wrapHtml(htmlContent, title)
	err = writeStringFile(zw, "OEBPS/Text/content.xhtml", finalHtml)
	if err != nil {
		return err
	}

	// TODO: 提取图片资源并写入 OEBPS/Images/
	// 需要解析 images 并替换 HTML 中的引用 (recindex)

	return nil
}

func writeStringFile(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, content)
	return err
}

func writeTemplateFile(zw *zip.Writer, name, tmplStr string, data interface{}) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	t, err := template.New(filepath.Base(name)).Parse(tmplStr)
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}

func wrapHtml(content, title string) string {
	// 简单的 XHTML 包装
	// 如果 content 已经包含 <html>，则可能需要解析和提取 body
	// 这里假设 content 是 body 内容或完整的 html
	if strings.Contains(strings.ToLower(content), "<html") {
		return content // 假设已经是完整的，或者是 KF8 的完整文档
	}
	return fmt.Sprintf(xhtmlTemplate, title, content)
}

// Templates

const containerXml = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
    <rootfiles>
        <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
    </rootfiles>
</container>`

type OpfData struct {
	Title      string
	Language   string
	Identifier string
	Date       string
}

const opfTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="BookId" version="2.0">
    <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
        <dc:title>{{.Title}}</dc:title>
        <dc:language>{{.Language}}</dc:language>
        <dc:identifier id="BookId" opf:scheme="UUID">{{.Identifier}}</dc:identifier>
        <dc:date>{{.Date}}</dc:date>
    </metadata>
    <manifest>
        <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
        <item id="content" href="Text/content.xhtml" media-type="application/xhtml+xml"/>
    </manifest>
    <spine toc="ncx">
        <itemref idref="content"/>
    </spine>
</package>`

type NcxData struct {
	Title      string
	Identifier string
}

const ncxTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
    <head>
        <meta name="dtb:uid" content="{{.Identifier}}"/>
        <meta name="dtb:depth" content="1"/>
        <meta name="dtb:totalPageCount" content="0"/>
        <meta name="dtb:maxPageNumber" content="0"/>
    </head>
    <docTitle>
        <text>{{.Title}}</text>
    </docTitle>
    <navMap>
        <navPoint id="navPoint-1" playOrder="1">
            <navLabel>
                <text>Start</text>
            </navLabel>
            <content src="Text/content.xhtml"/>
        </navPoint>
    </navMap>
</ncx>`

const xhtmlTemplate = `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<title>%s</title>
</head>
<body>
%s
</body>
</html>`
