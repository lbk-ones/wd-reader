package _type

import "encoding/xml"

// NCX 是根元素，对应 <ncx> 元素
type NCX struct {
	XMLName   xml.Name  `xml:"ncx"`
	XMLNS     string    `xml:"xmlns,attr"`
	Lang      string    `xml:"xml:lang,attr"`
	Head      Head      `xml:"head"`
	DocTitle  DocTitle  `xml:"docTitle"`
	DocAuthor DocAuthor `xml:"docAuthor"`
	NavMap    NavMap    `xml:"navMap"`
}

// Head 对应 <head> 元素
type Head struct {
	Meta []NcMeta `xml:"meta"`
}

// NcMeta 对应 <meta> 元素
type NcMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

// DocTitle 对应 <docTitle> 元素
type DocTitle struct {
	Text string `xml:"text"`
}

// DocAuthor 对应 <docAuthor> 元素
type DocAuthor struct {
	Text string `xml:"text"`
}

// NavMap 对应 <navMap> 元素
type NavMap struct {
	NavPoints []NavPoint `xml:"navPoint"`
}

// NavPoint 对应 <navPoint> 元素
type NavPoint struct {
	ID        string     `xml:"id,attr"`
	PlayOrder int        `xml:"playOrder,attr"`
	NavLabel  NavLabel   `xml:"navLabel"`
	Content   NavContent `xml:"content"`
	Points    []NavPoint `xml:"navPoint"`
}

// NavLabel 对应 <navLabel> 元素
type NavLabel struct {
	Text string `xml:"text"`
}

// NavContent 对应 <content> 元素
type NavContent struct {
	Src string `xml:"src,attr"`
}
