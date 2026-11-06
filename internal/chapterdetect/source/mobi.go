package source

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// positionLink 保存原始文本中的目录链接位置，偏移均按未经注入标记的字节流计算。
type positionLink struct {
	position, target int    // position 为链接自身的字节偏移；target 为链接指向的字节偏移。
	title            string // 链接内部可见文字，空白归一化后用作目录标题。
}

// filePositionTOC 仅信任原书导引明确指定的目录区域内的字节位置链接。
// 调用方必须确认文本仍保持原始 UTF-8 字节坐标；其他编码转换后不能沿用这些偏移。
// 返回注入定位标记后的文本和标记到标题的映射；目录无效时保留原文并返回空映射。
func filePositionTOC(raw string) (string, map[string][]string) {
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	position, tocStart := 0, -1
	var links []positionLink
	var active *positionLink
	for {
		kind := tokenizer.Next()
		start := position
		// 使用分词器消费的原始字节长度累计偏移，不能用解码后的标签文字长度替代。
		position += len(tokenizer.Raw())
		if kind == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			attributes := map[string]string{}
			for _, a := range token.Attr {
				attributes[a.Key] = a.Val
			}
			if token.Data == "reference" && attributes["type"] == "toc" {
				if offset, err := strconv.Atoi(attributes["filepos"]); err == nil {
					tocStart = offset
				}
			}
			if token.Data == "a" && attributes["filepos"] != "" {
				if offset, err := strconv.Atoi(attributes["filepos"]); err == nil {
					active = &positionLink{position: start, target: offset}
				}
			}
		} else if kind == html.TextToken && active != nil {
			active.title += token.Data
		} else if kind == html.EndTagToken && token.Data == "a" && active != nil {
			active.title = strings.Join(strings.Fields(active.title), " ")
			links = append(links, *active)
			active = nil
		}
	}
	targets := map[string][]string{}
	if tocStart < 0 || tocStart >= len(raw) {
		return raw, targets
	}
	// 目录之后最早的正文目标作为目录区域上界，限制普通正文链接参与目录判断。
	tocEnd := len(raw)
	for _, entry := range links {
		if entry.target > tocStart && entry.target < tocEnd {
			tocEnd = entry.target
		}
	}
	// 目标必须位于有效字节范围并指向标签起始处；不猜测偏移附近的实际位置。
	positions := map[int]bool{}
	for _, entry := range links {
		if entry.position < tocStart || entry.position >= tocEnd || entry.target < 0 || entry.target >= len(raw) ||
			raw[entry.target] != '<' || entry.title == "" {
			continue
		}
		id := fmt.Sprintf("wd-native-filepos-%d", entry.target)
		targets[id] = append(targets[id], entry.title)
		positions[entry.target] = true
	}
	ordered := make([]int, 0, len(positions))
	for offset := range positions {
		ordered = append(ordered, offset)
	}
	// 按原始偏移从前往后拼接并插入不可见锚点，插入内容不会改变后续原始切片的坐标。
	sort.Ints(ordered)
	var output strings.Builder
	last := 0
	for _, offset := range ordered {
		output.WriteString(raw[last:offset])
		fmt.Fprintf(&output, `<span id="wd-native-filepos-%d"></span>`, offset)
		last = offset
	}
	output.WriteString(raw[last:])
	return output.String(), targets
}

// ReadMOBIHTML 从已解压的电子书文本提取正文，识别命名目录以及可验证的 UTF-8 字节位置目录。
// utf8FilePositions 表明原始编码允许可靠使用字节偏移；为假时只使用可定位的命名锚点。
// 仅含二进制导航且无法可靠映射的书籍不猜测位置，保留正文供上层进行文本检测。
func ReadMOBIHTML(reader io.Reader, utf8FilePositions bool) (Document, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (512<<20)+1))
	if err != nil || len(data) > 512<<20 {
		return Document{}, fmt.Errorf("MOBI 正文读取失败或超过解析限制")
	}
	raw := string(data)
	if !strings.Contains(raw, "<") {
		// 无标签的原始内容按纯文本处理，保留物理空行和缩进，避免丢失后续检测所需的上下文。
		raw = strings.TrimPrefix(strings.ReplaceAll(raw, "\r\n", "\n"), "\ufeff")
		lines := strings.Split(strings.TrimRight(raw, "\r\n"), "\n")
		if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
			return Document{}, fmt.Errorf("MOBI 未提取到正文")
		}
		return Document{Lines: lines}, nil
	}
	// 先处理字节坐标，再解析命名锚点；前一步只在原编码可信时执行。
	targets := map[string][]string{}
	if utf8FilePositions {
		raw, targets = filePositionTOC(raw)
	}
	// 文档树解析失败时，目录链接收集和正文渲染均使用流式实现，兼容大量未闭合标签。
	doc, parseErr := html.Parse(strings.NewReader(raw))
	var links []link
	if parseErr == nil {
		links = tocLinks(doc)
	} else {
		links = streamTOCLinks(raw)
	}
	// 命名目录只接受书内片段链接，外部地址不参与章节定位。
	for _, entry := range links {
		u, err := url.Parse(entry.href)
		if err == nil && u.Scheme == "" && u.Host == "" && u.Fragment != "" {
			targets[u.Fragment] = append(targets[u.Fragment], entry.title)
		}
	}
	builder := &textBuilder{}
	if parseErr == nil {
		renderHTML(builder, doc, targets, true)
	} else {
		renderHTMLStream(builder, raw, targets)
	}
	result := builder.finish()
	if len(result.Lines) == 0 {
		return Document{}, fmt.Errorf("MOBI 未提取到正文")
	}
	return result, nil
}
