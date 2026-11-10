package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	dcNamespace      = "http://purl.org/dc/elements/1.1/"
	opfNamespace     = "http://www.idpf.org/2007/opf"
	sourceDigestMeta = "wd-reader:merge-source-sha256"
)

// writeEdition 只重写默认 OPF。其余 ZIP 条目原样复制，保留 EPUB 的正文、封面、目录和资源。
func writeEdition(out io.Writer, book edition) error {
	archive, err := zip.OpenReader(book.path)
	if err != nil {
		return err
	}
	defer archive.Close()
	data, err := readMetadataEntry(archive, book.packagePath)
	if err != nil {
		return err
	}
	digest, ok := strings.CutPrefix(book.key, "file:")
	if !ok {
		digest = ""
	}
	data, err = rewritePackage(data, book.category, book.channel, digest)
	if err != nil {
		return fmt.Errorf("%s: %w", book.packagePath, err)
	}
	writer := zip.NewWriter(out)
	if err := writer.SetComment(archive.Comment); err != nil {
		return err
	}
	for _, entry := range archive.File {
		if entry.Name != book.packagePath {
			if err := writer.Copy(entry); err != nil {
				return err
			}
			continue
		}
		header := entry.FileHeader
		// OPF 体积小，使用不压缩的条目让重写结果不依赖压缩器版本。
		header.Method = zip.Store
		header.Flags = 0   // 由 ZIP 写入器根据名称和新条目重新设置标记。
		header.Extra = nil // 旧的扩展头可能包含原条目的大小等信息。
		file, err := writer.CreateHeader(&header)
		if err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			return err
		}
	}
	return writer.Close()
}

type opfElement struct {
	name       xml.Name
	attrs      []xml.Attr
	start, end int
	value      string
	remove     bool
}

func (e opfElement) attr(key string) string {
	for _, attr := range e.attrs {
		if attr.Name.Space == "" && attr.Name.Local == key {
			return attr.Value
		}
	}
	return ""
}

func (e opfElement) isMeta() bool {
	return e.name.Local == "meta" && (e.name.Space == opfNamespace || e.name.Space == "")
}

// rewritePackage 使用 XML 解析器定位元数据，再按字节范围改写，保留原命名空间、清单和阅读顺序。
func rewritePackage(data []byte, category, channel, digest string) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var depth int
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("EPUB 缺少 metadata 元素")
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 1 && token.Name.Local == "metadata" {
				elements, end, err := readMetadataElements(decoder)
				if err != nil {
					return nil, err
				}
				return replaceClassification(data, elements, end, category, channel, digest), nil
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

func readMetadataElements(decoder *xml.Decoder) ([]opfElement, int, error) {
	var elements []opfElement
	for {
		start := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err != nil {
			return nil, 0, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			element := opfElement{name: token.Name, attrs: token.Attr, start: start}
			if err := decoder.DecodeElement(&element.value, &token); err != nil {
				return nil, 0, err
			}
			element.end = int(decoder.InputOffset())
			elements = append(elements, element)
		case xml.EndElement:
			if start == int(decoder.InputOffset()) {
				return nil, 0, fmt.Errorf("EPUB 的 metadata 元素为空")
			}
			return elements, start, nil
		}
	}
}

func replaceClassification(data []byte, elements []opfElement, end int, category, channel, digest string) []byte {
	removedIDs := make(map[string]bool)
	for i := range elements {
		e := &elements[i]
		dc := e.name.Space == dcNamespace || e.name.Space == ""
		oldClassification := dc && (e.name.Local == "subject" || e.name.Local == "type" && normalizeChannel(e.value) != "")
		e.remove = oldClassification || e.isMeta() && e.attr("name") == sourceDigestMeta
		if id := e.attr("id"); e.remove && id != "" {
			removedIDs["#"+id] = true
		}
	}
	// 旧分类关联的编码和细化说明也需要删除，避免留下指向旧 subject 的引用。
	for changed := len(removedIDs) > 0; changed; {
		changed = false
		for i := range elements {
			e := &elements[i]
			if !e.remove && e.isMeta() && removedIDs[e.attr("refines")] {
				e.remove = true
				if id := e.attr("id"); id != "" {
					removedIDs["#"+id] = true
					changed = true
				}
			}
		}
	}
	var out bytes.Buffer
	last := 0
	for _, e := range elements {
		if e.remove {
			out.Write(data[last:e.start])
			last = e.end
		}
	}
	out.Write(data[last:end])
	writeDC := func(key, value string) {
		fmt.Fprintf(&out, "\n  <dc:%s xmlns:dc=\"%s\">", key, dcNamespace)
		_ = xml.EscapeText(&out, []byte(value))
		fmt.Fprintf(&out, "</dc:%s>", key)
	}
	writeDC("subject", category)
	writeDC("type", channel)
	if digest != "" {
		fmt.Fprintf(&out, "\n  <meta xmlns=\"%s\" name=\"%s\" content=\"%s\"/>", opfNamespace, sourceDigestMeta, digest)
	}
	out.WriteByte('\n')
	out.Write(data[end:])
	return out.Bytes()
}
