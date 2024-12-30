package main

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Item 定义与 XML 元素对应的结构体
type Item struct {
	XMLName   xml.Name `xml:"item"`
	ID        string   `xml:"id,attr"`
	Href      string   `xml:"href,attr"`
	MediaType string   `xml:"media-type,attr"`
}

func main() {
	xmlStr := `
    <item id="chapter-8.xhtml" href="Text/chapter-8.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-7.xhtml" href="Text/chapter-7.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter-6.xhtml" href="Text/chapter-6.xhtml" media-type="application/xhtml+xml"/>
    `
	// 存储解析结果的切片
	var items []Item

	decoder := xml.NewDecoder(strings.NewReader(xmlStr))
	for {
		var item Item
		err := decoder.Decode(&item)
		if err != nil {
			break
		}
		items = append(items, item)
	}

	// 打印解析结果
	for _, item := range items {
		fmt.Printf("ID: %s, Href: %s, MediaType: %s\n", item.ID, item.Href, item.MediaType)
	}
}
