// Package source 在提取电子书正文时保留可可靠定位的原生目录，并将文件片段映射为物理行号。
//
// 格式解析流程：读取原生目录链接，按正文阅读顺序渲染文本，在命中链接目标时记录锚点，
// 最后清理空行定位并合并同位置条目，输出正文与同一坐标系中的目录。
// 正常文件使用文档树解析；标签异常或嵌套过深时使用流式解析保留正文。
// 本包不从任意标题元素或普通超链接推断章节，文本特征判断由上层 chapterdetect 完成。
package source

import (
	"sort"
	"strings"
	"unicode"

	"wd-reader/internal/chapterdetect"

	"golang.org/x/net/html"
)

// Document 是格式解码后的统一结果，正文与原生目录行号必须保持一致。
type Document struct {
	Lines  []string               // 按阅读顺序输出的物理行，包含段落间空行。
	Native []chapterdetect.Anchor // 可定位的原生目录锚点；缺失时由上层进行文本检测。
}

// Text 将物理行序列化为以换行结尾的文本，供书库保存，与逐行扫描的索引坐标对应。
func (d Document) Text() string { return strings.Join(d.Lines, "\n") + "\n" }

// textBuilder 为文档树与流式解析共用的正文构建器，统一段落、空白及锚点坐标规则。
type textBuilder struct {
	lines   []string               // 已完成的物理行，空字符串表示段落留白。
	line    strings.Builder        // 尚未提交的当前可见文本行。
	space   bool                   // 当前行后是否待补一个规范化空格，避免行内元素文字粘连。
	anchors []chapterdetect.Anchor // 渲染途中记录的目录位置，完成后还需校正和去重。
}

// flush 提交当前非空行并重置行缓冲；不主动生成空白行。
func (b *textBuilder) flush() {
	if b.line.Len() > 0 {
		b.lines = append(b.lines, b.line.String())
		b.line.Reset()
	}
	b.space = false
}

// paragraph 结束当前段落，并保留至多一个段落间空行，供后续上下文判断使用。
func (b *textBuilder) paragraph() {
	b.flush()
	if len(b.lines) > 0 && b.lines[len(b.lines)-1] != "" {
		b.lines = append(b.lines, "")
	}
}

// text 追加可见文字，合并普通排版空白；预格式化区域中的换行会直接提交当前行。
// 空格延迟到下一个可见字符前写入，避免产生前导空格、尾随空格或丢失行内词间距。
func (b *textBuilder) text(text string, preformatted bool) {
	for _, r := range text {
		if preformatted && r == '\n' {
			b.flush()
			continue
		}
		if unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff' {
			b.space = b.line.Len() > 0
			continue
		}
		if b.space {
			b.line.WriteByte(' ')
			b.space = false
		}
		b.line.WriteRune(r)
	}
}

// mark 在下一个可见行之前记录目录锚点；先提交当前行，避免锚点落在上一段文字内部。
func (b *textBuilder) mark(title string) {
	b.flush()
	if title = strings.TrimSpace(title); title != "" {
		b.anchors = append(b.anchors, chapterdetect.Anchor{Line: len(b.lines), Title: title})
	}
}

// finishAnchors 在单个正文文件渲染完毕后校正该文件新增加的锚点。
// start 是本文件开始前的锚点数量，已完成文件的锚点不参与当前文件的修正。
func (b *textBuilder) finishAnchors(start int) {
	// 只允许跳过当前文件内的空行；纯图片页或文件末尾的锚点不得漂移到下一正文文件。
	valid := b.anchors[:start]
	for _, anchor := range b.anchors[start:] {
		for anchor.Line < len(b.lines) && chapterdetect.TextKey(b.lines[anchor.Line]) == "" {
			anchor.Line++
		}
		if anchor.Line < len(b.lines) {
			valid = append(valid, anchor)
		}
	}
	b.anchors = valid
}

// attr 获取元素属性，并兼容解析器保留完整前缀形式的电子书类型属性。
func attr(node *html.Node, name string) string {
	for _, value := range node.Attr {
		if value.Key == name || (name == "type" && value.Key == "epub:type") {
			return value.Val
		}
	}
	return ""
}

// nodeText 递归提取目录链接中的所有文字，将嵌套行内元素的文本合并为一个标题。
func nodeText(node *html.Node) string {
	var text strings.Builder
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return strings.Join(strings.Fields(text.String()), " ")
}

// isTOC 仅识别具有明确语义标记或约定标识的目录区域，避免把正文链接当作目录。
func isTOC(node *html.Node) bool {
	if node.Type != html.ElementNode {
		return false
	}
	if node.Data == "nav" && (strings.Contains(" "+attr(node, "type")+" ", " toc ") || attr(node, "role") == "doc-toc") {
		return true
	}
	id := strings.ToLower(attr(node, "id"))
	return id == "toc" || id == "tableofcontents" || id == "table-of-contents" || id == "contents"
}

// link 表示尚未映射到正文位置的原始目录链接。
type link struct {
	href  string // 目标文件路径及可选片段，格式解析器负责相对路径解析。
	title string // 链接展示文字，作为章节目录标题。
}

// tocLinks 遍历文档树，只收集明确目录区域中的超链接，保留原始遍历顺序。
func tocLinks(doc *html.Node) []link {
	var links []link
	var visit func(*html.Node, bool)
	visit = func(node *html.Node, inside bool) {
		inside = inside || isTOC(node)
		if inside && node.Type == html.ElementNode && node.Data == "a" && attr(node, "href") != "" {
			links = append(links, link{attr(node, "href"), nodeText(node)})
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child, inside)
		}
	}
	visit(doc, false)
	return links
}

// renderHTML 按文档顺序提取可见正文，并在目标元素处将目录标题转换为行号锚点。
// targets 的键为片段标识，空键表示整个文件入口；skipTOC 为真时仅跳过目录区域，保留同文件正文。
func renderHTML(builder *textBuilder, doc *html.Node, targets map[string][]string, skipTOC bool) {
	anchorStart := len(builder.anchors)
	for _, title := range targets[""] {
		builder.mark(title)
	}
	var visit func(*html.Node, bool)
	visit = func(node *html.Node, preformatted bool) {
		if skipTOC && isTOC(node) {
			return
		}
		if node.Type == html.ElementNode {
			switch node.Data {
			case "head", "script", "style", "noscript", "template":
				return
			}
		}
		// 块级元素两侧形成段落边界；行内元素只拼接文字，避免把标题中的强调文字拆成多行。
		block := node.Type == html.ElementNode && blockToken(node.Data)
		if block {
			builder.paragraph()
		}
		if node.Type == html.ElementNode {
			// 同时支持元素标识和旧式命名锚点；相同标识只记录一次。
			ids := []string{attr(node, "id")}
			if node.Data == "a" && attr(node, "name") != ids[0] {
				ids = append(ids, attr(node, "name"))
			}
			for _, id := range ids {
				if id != "" {
					for _, title := range targets[id] {
						builder.mark(title)
					}
				}
			}
			if node.Data == "br" || node.Data == "hr" || node.Data == "pagebreak" {
				builder.paragraph()
			}
			preformatted = preformatted || node.Data == "pre"
		} else if node.Type == html.TextNode {
			builder.text(node.Data, preformatted)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child, preformatted)
		}
		if block {
			builder.paragraph()
		}
	}
	visit(doc, false)
	builder.paragraph()
	builder.finishAnchors(anchorStart)
}

// finish 完成整本书的构建，清理末尾留白并将有效锚点整理为严格递增的目录。
func (b *textBuilder) finish() Document {
	b.flush()
	for len(b.lines) > 0 && b.lines[len(b.lines)-1] == "" {
		b.lines = b.lines[:len(b.lines)-1]
	}
	byLine := map[int]chapterdetect.Anchor{}
	for _, anchor := range b.anchors {
		// 元素标记可能落在块边界的空行前，应定位到其后的首个可见行。
		for anchor.Line < len(b.lines) && chapterdetect.TextKey(b.lines[anchor.Line]) == "" {
			anchor.Line++
		}
		if anchor.Line >= len(b.lines) {
			continue
		}
		// 目录指向正文段落时标记保留原行；同一位置的多个条目优先选择与实际标题行一致的条目。
		anchor.KeepLine = chapterdetect.TextKey(b.lines[anchor.Line]) != chapterdetect.TextKey(anchor.Title)
		previous, exists := byLine[anchor.Line]
		if !exists || previous.KeepLine || !anchor.KeepLine {
			byLine[anchor.Line] = anchor
		}
	}
	anchors := make([]chapterdetect.Anchor, 0, len(byLine))
	for _, anchor := range byLine {
		anchors = append(anchors, anchor)
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].Line < anchors[j].Line })
	return Document{Lines: b.lines, Native: anchors}
}
