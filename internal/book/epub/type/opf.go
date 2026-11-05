package _type

import "encoding/xml"

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
	Language    string     `xml:"http://purl.org/dc/elements/1.1/ language"`
	Creator     []string   `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Title       string     `xml:"http://purl.org/dc/elements/1.1/ title"`
	Description string     `xml:"http://purl.org/dc/elements/1.1/ description"`
	Publisher   string     `xml:"http://purl.org/dc/elements/1.1/ publisher"`
	Meta        []OpMeta   `xml:"meta"`
	Date        Date       `xml:"http://purl.org/dc/elements/1.1/ date"`
	Identifier  Identifier `xml:"http://purl.org/dc/elements/1.1/ identifier"`
}

// OpMeta 结构体对应 <meta> 元素
type OpMeta struct {
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
