package source

import (
	"strings"

	"golang.org/x/net/html"
)

// tokenNode 为单个开始标签构造轻量节点，复用属性读取和目录区域识别规则。
func tokenNode(token html.Token) *html.Node {
	return &html.Node{Type: html.ElementNode, Data: token.Data, Attr: token.Attr}
}

// streamTOCLinks 在无法构建完整文档树时，从明确且完整闭合的目录区域收集链接。
// 某些电子书存在大量未闭合容器；未闭合的目录区域不能把其后整本正文链接都当作目录。
func streamTOCLinks(raw string) []link {
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	var result, regionLinks []link
	var active *link
	// 只跟踪当前目录容器同名标签的嵌套深度；区域闭合后才提交暂存链接。
	region, depth := "", 0
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if kind == html.StartTagToken {
			if region == "" && isTOC(tokenNode(token)) {
				region, depth = token.Data, 1
				regionLinks = nil
			} else if token.Data == region {
				depth++
			}
			if region != "" && token.Data == "a" && attr(tokenNode(token), "href") != "" {
				active = &link{href: attr(tokenNode(token), "href")}
			}
		} else if kind == html.TextToken && active != nil {
			active.title += token.Data
		} else if kind == html.EndTagToken {
			if token.Data == "a" && active != nil {
				active.title = strings.Join(strings.Fields(active.title), " ")
				regionLinks = append(regionLinks, *active)
				active = nil
			}
			if token.Data == region {
				depth--
				if depth == 0 {
					// 此处才确认区域完整；文件结束时尚未闭合的暂存链接会直接被忽略。
					result = append(result, regionLinks...)
					region = ""
					active = nil
				}
			}
		}
	}
	return result
}

// renderHTMLStream 按标签流提取正文，避免异常嵌套深度使整本书无法解析。
// targets 与文档树渲染使用相同的片段坐标规则；不推断未闭合目录的结束范围，优先保留可见文字。
// 每个文件结束时立即收束本文件锚点，避免无正文锚点漂移到下一文件。
func renderHTMLStream(builder *textBuilder, raw string, targets map[string][]string) {
	anchorStart := len(builder.anchors)
	for _, title := range targets[""] {
		builder.mark(title)
	}
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	// ignored 记录当前应跳过的非正文区域；preformatted 控制预格式化文字的换行保留。
	ignored, preformatted := "", false
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			if token.Data == "body" {
				ignored = "" // 正文起始标签隐式结束异常未闭合的头部，防止随后正文被整段跳过。
			}
			if ignored != "" {
				continue
			}
			switch token.Data {
			case "head", "script", "style", "noscript", "template":
				if kind != html.SelfClosingTagToken {
					ignored = token.Data
				}
				continue
			}
			// 只将明确块级或换行标签当作段落边界；图片等空元素不会打开持续忽略状态。
			if blockToken(token.Data) || token.Data == "br" || token.Data == "pagebreak" {
				builder.paragraph()
			}
			node := tokenNode(token)
			// 兼容元素标识与旧式命名锚点，将命中的原生目录记录到当前输出行。
			ids := []string{attr(node, "id")}
			if token.Data == "a" && attr(node, "name") != ids[0] {
				ids = append(ids, attr(node, "name"))
			}
			for _, id := range ids {
				if id != "" {
					for _, title := range targets[id] {
						builder.mark(title)
					}
				}
			}
			if token.Data == "pre" {
				preformatted = true
			}
		} else if kind == html.EndTagToken {
			if ignored != "" {
				if token.Data == ignored {
					ignored = ""
				}
				continue
			}
			if blockToken(token.Data) {
				builder.paragraph()
			}
			if token.Data == "pre" {
				preformatted = false
			}
		} else if kind == html.TextToken && ignored == "" {
			builder.text(token.Data, preformatted)
		}
	}
	builder.paragraph()
	builder.finishAnchors(anchorStart)
}

// blockToken 判断需要在两侧形成段落边界的块级标签，为两种渲染方式提供一致规则。
func blockToken(tag string) bool {
	return strings.Contains("|p|div|h1|h2|h3|h4|h5|h6|section|article|li|blockquote|pre|tr|", "|"+tag+"|")
}
