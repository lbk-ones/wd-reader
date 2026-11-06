package source

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// HTMLSection 表示格式适配层已经恢复阅读顺序及原生目录目标的独立文档。
// 解析器逐个处理文档，避免直接拼接多个完整网页导致后续头部标题混入正文。
type HTMLSection struct {
	HTML     string              // 当前文档的完整标记文本。
	Targets  map[string][]string // 已验证目标标识到原生目录标题的映射。
	TOCStart string              // 导引明确给出的目录起始标识，为空时不推断普通段落是否属于目录。
}

// ReadHTMLSections 按阅读顺序提取多个已定位文档，输出与原生目录共用行号坐标的正文。
// 单个文档解析失败时继续使用流式提取，目录边界只在证据明确时隐藏，其他文字一律保留。
func ReadHTMLSections(sections []HTMLSection) (Document, error) {
	builder := &textBuilder{}
	for _, section := range sections {
		doc, err := html.Parse(strings.NewReader(section.HTML))
		if err != nil {
			renderHTMLStream(builder, section.HTML, section.Targets)
			continue
		}
		if section.TOCStart != "" {
			markGuidedTOC(doc, section.TOCStart)
		}
		renderHTML(builder, doc, section.Targets, true)
	}
	result := builder.finish()
	if len(result.Lines) == 0 {
		return Document{}, fmt.Errorf("未提取到可阅读正文")
	}
	return result, nil
}

// markGuidedTOC 将原书明确导引位置之后连续的目录标题及目录链接包入目录节点。
// 遇到普通文字即结束，保留同文档中目录之后的附录或正文，不以整份文件作为删除范围。
func markGuidedTOC(doc *html.Node, id string) {
	var marker *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if attr(n, "id") == id {
			marker = n
			return
		}
		for child := n.FirstChild; child != nil && marker == nil; child = child.NextSibling {
			find(child)
		}
	}
	find(doc)
	if marker == nil || marker.Parent == nil {
		return
	}
	nodes := []*html.Node{marker}
	links := 0
	for next := marker.NextSibling; next != nil; next = next.NextSibling {
		count, valid := guidedDirectoryNode(next, false)
		if !valid {
			break
		}
		links += count
		nodes = append(nodes, next)
	}
	if links == 0 {
		return
	}
	parent := marker.Parent
	wrapper := &html.Node{Type: html.ElementNode, Data: "nav", Attr: []html.Attribute{{Key: "role", Val: "doc-toc"}}}
	parent.InsertBefore(wrapper, marker)
	for _, node := range nodes {
		parent.RemoveChild(node)
		wrapper.AppendChild(node)
	}
}

// guidedDirectoryNode 验证目录区域中的子树只含目录标题、留白及书内位置链接。
// 此判断只用于原书导引指定区域，正文其他位置中的同类链接不会被收集或隐藏。
func guidedDirectoryNode(node *html.Node, insideLink bool) (int, bool) {
	if node.Type == html.TextNode {
		text := strings.ToLower(strings.Join(strings.Fields(node.Data), ""))
		return 0, insideLink || text == "" || text == "目录" || text == "contents" || text == "tableofcontents"
	}
	count := 0
	if node.Type == html.ElementNode {
		if node.Data == "a" {
			href := strings.ToLower(attr(node, "href"))
			if !strings.HasPrefix(href, "kindle:pos:fid:") && !strings.HasPrefix(href, "#") {
				return 0, false
			}
			insideLink = true
			count = 1
		} else if node.Data == "img" || node.Data == "script" || node.Data == "style" {
			return 0, false
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		n, valid := guidedDirectoryNode(child, insideLink)
		if !valid {
			return 0, false
		}
		count += n
	}
	return count, true
}
