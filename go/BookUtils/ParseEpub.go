package BookUtils

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"wd-reader/go/constant"
)

// Package 是最外层的结构体，对应 XML 的 <package> 元素
type Package struct {
	XMLName  xml.Name `xml:"package"`
	Version  string   `xml:"version,attr"`
	UniqueID string   `xml:"unique-identifier,attr"`
	Metadata Metadata `xml:"metadata"`
	Manifest Manifest `xml:"manifest"`
	Spine    Spine    `xml:"spine"`
	Guide    Guide    `xml:"guide"`
}

// Metadata 结构体对应 <metadata> 元素
type Metadata struct {
	XMLName xml.Name `xml:"metadata"`
	// xmlns 命名空间相关信息会被忽略，因为 Go 的 xml 包会自动处理命名空间
	Language   string     `xml:"http://purl.org/dc/elements/1.1/ language"`
	Creator    string     `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Title      string     `xml:"http://purl.org/dc/elements/1.1/ title"`
	Meta       []Meta     `xml:"meta"`
	Date       Date       `xml:"http://purl.org/dc/elements/1.1/ date"`
	Identifier Identifier `xml:"http://purl.org/dc/elements/1.1/ identifier"`
}

// Meta 结构体对应 <meta> 元素
type Meta struct {
	XMLName xml.Name `xml:"meta"`
	Content string   `xml:"content,attr"`
	Name    string   `xml:"name,attr"`
}

// Date 结构体对应 <dc:date> 元素
type Date struct {
	XMLName xml.Name `xml:"http://purl.org/dc/elements/1.1/ date"`
	Event   string   `xml:"event,attr"`
	Value   string   `xml:",chardata"`
}

// Identifier 结构体对应 <dc:identifier> 元素
type Identifier struct {
	XMLName xml.Name `xml:"http://purl.org/dc/elements/1.1/ identifier"`
	ID      string   `xml:"id,attr"`
	Scheme  string   `xml:"scheme,attr"`
	Value   string   `xml:",chardata"`
}

// Manifest 结构体对应 <manifest> 元素
type Manifest struct {
	XMLName xml.Name `xml:"manifest"`
	Items   []Item   `xml:"item"`
}

// Item 结构体对应 <item> 元素
type Item struct {
	XMLName   xml.Name `xml:"item"`
	ID        string   `xml:"id,attr"`
	Href      string   `xml:"href,attr"`
	MediaType string   `xml:"media-type,attr"`
}

// Spine 结构体对应 <spine> 元素
type Spine struct {
	XMLName  xml.Name  `xml:"spine"`
	TOC      string    `xml:"toc,attr"`
	ItemRefs []ItemRef `xml:"itemref"`
}

// ItemRef 结构体对应 <itemref> 元素
type ItemRef struct {
	XMLName    xml.Name `xml:"itemref"`
	IDRef      string   `xml:"idref,attr"`
	Properties string   `xml:"properties,attr"`
}

// Guide 结构体对应 <guide> 元素
type Guide struct {
	XMLName    xml.Name    `xml:"guide"`
	References []Reference `xml:"reference"`
}

// Reference 结构体对应 <reference> 元素
type Reference struct {
	XMLName xml.Name `xml:"reference"`
	Type    string   `xml:"type,attr"`
	Title   string   `xml:"title,attr"`
	Href    string   `xml:"href,attr"`
}

// ParseEpubToTxt 解析epub文件生成对应的txt文件
func ParseEpubToTxt(filename string) string {
	dir := GetAppPath()
	join := filepath.Join(dir, constant.BOOK_PATH, filename)
	_, err2 := os.Stat(join)
	if os.IsNotExist(err2) {
		return WrapperException(err2)
	}
	replace := strings.Replace(join, ".epub", ".txt", -1)
	_, err2 = os.Stat(replace)
	if os.IsExist(err2) {
		return WrapperExceptionStr("已经存在不用初始化")
	}
	epubFile := join
	outputTxt := replace

	// 打开 EPUB 文件
	zipReader, err := zip.OpenReader(epubFile)
	if err != nil {
		return WrapperException(err)
	}
	defer zipReader.Close()

	var opfFile string
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".opf") {
			opfFile = f.Name
			break
		}
	}
	if opfFile == "" {
		fmt.Println("OPF file not found")
		return WrapperExceptionStr("OPF file not found")
	}

	// 读取 OPF 文件
	opfReader, err := zipReader.Open(opfFile)
	if err != nil {
		return WrapperExceptionStr(fmt.Sprint("Error opening OPF file:", err))
	}
	defer opfReader.Close()

	all, _ := io.ReadAll(opfReader)
	var opf Package
	err = xml.Unmarshal(all, &opf)
	if err != nil {
		return WrapperExceptionStr(fmt.Sprint("Error parsing OPF file:", err))
	}
	file, err := os.Create(outputTxt)
	if err != nil {
		return WrapperExceptionStr(fmt.Sprint("Error creating output file:", err))
	}
	defer file.Close()

	var chapterIndex = 1
	for _, itemRef := range opf.Spine.ItemRefs {
		var chapterFile string
		for _, item := range opf.Manifest.Items {
			if item.ID == itemRef.IDRef {
				if strings.Contains(item.MediaType, "html") || strings.Contains(item.MediaType, "HTML") {
					chapterFile = item.Href
					break
				}
			}
		}
		if chapterFile == "" {
			fmt.Printf("Chapter file for itemref %s not found\n", itemRef)
			continue
		}

		// 处理相对路径
		chapterPath := filepath.Dir(opfFile) + "/" + chapterFile
		chapterZip, err := zipReader.Open(chapterPath)
		if err != nil {
			fmt.Printf("Error opening chapter file %s: %v\n", chapterFile, err)
			continue
		}
		defer chapterZip.Close()

		doc, err := html.Parse(chapterZip)
		if err != nil {
			fmt.Printf("Error parsing chapter file %s: %v\n", chapterFile, err)
			continue
		}

		// 提取文本内容 每一个完整的节点下面不应该有换行
		var extractText func(*html.Node) string
		extractText = func(n *html.Node) string {
			var s string
			if n.Type == html.ElementNode && (n.Data == "title" || n.Data == "script" || n.Data == "style" || n.Data == "noscript" || n.Data == "meta") {
				return ""
			} else if n.Type == html.TextNode {
				s = n.Data
			}
			var tag string
			if n.Type == html.ElementNode {
				tag = n.Data
			}
			// 递归
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				s += extractText(c)
			}
			// 这些元素不能跨行
			if tag == "p" || tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" {
				s = strings.ReplaceAll(s, "\n", "") + "\n"
			}
			// 处理章节
			if tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" {
				compile := regexp.MustCompile(`第([ ,、一二三四五六七八九十零百千万亿\d]+)[章巻]`)
				findString := compile.FindString(s)
				if findString == "" {
					s = strings.ReplaceAll(fmt.Sprint("第", chapterIndex, "章", " ", s), "\n", "") + "\n"
				}
			}

			return s
		}

		text := extractText(doc)

		re := regexp.MustCompile(`\n+`)
		result := re.ReplaceAllString(text, "\n")
		//fmt.Println(text)
		_, _ = file.WriteString(result)
		chapterIndex++
	}

	return "ok"
}
