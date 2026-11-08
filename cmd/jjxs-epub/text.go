package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"

	"wd-reader/internal/epubexport"
	"wd-reader/internal/resourcecatalog"
)

func (s *source) downloadTXT(ctx context.Context, b book) (string, error) {
	if b.download == "" {
		return "", fmt.Errorf("书籍没有 TXT 下载页")
	}
	doc, err := s.page(ctx, b.download)
	if err != nil {
		return "", err
	}
	var link string
	for n := range doc.Descendants() {
		href := attr(n, "href")
		if href == "" {
			continue
		}
		u, err := url.Parse(href)
		if err != nil || u.Scheme != "https" || u.Host != "jjxswshuku.com" || u.User != nil ||
			u.Path != "/e/DownSys/doaction.php" {
			continue
		}
		if query := u.Query(); query.Get("id") == b.id && query.Get("pathid") == "0" {
			link = href
			break
		}
	}
	if link == "" {
		return "", fmt.Errorf("TXT 下载页没有可用链接")
	}
	dir, err := os.MkdirTemp("", "jjxs-epub-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dir)
	if err := s.wait(ctx); err != nil {
		return "", err
	}
	path, err := resourcecatalog.Download(ctx, s.client, resourcecatalog.File{
		URL: link, Name: "book.txt", Referer: s.base.String() + b.download,
	}, dir, nil)
	if err != nil {
		return "", err
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return epubexport.DecodeTXT(data)
}

var (
	generatedSection = regexp.MustCompile(`^(.+?)\s+第([1-9][0-9]*)节$`)
	decoratedHeading = regexp.MustCompile(`^☆\.\s*(第[0-9０-９]+章\s*[:：]\s*\S.*)$`)
	titleBreak       = regexp.MustCompile(`(?i)<br\s*/?>`)
)

func prepareTXT(text, bookTitle string, toc []chapter) ([]chapter, [][]string, error) {
	bodies, err := splitTXT(text, bookTitle, toc)
	if err == nil {
		return toc, bodies, nil
	}
	if len(toc) < 2 {
		return toc, bodies, err
	}
	first := generatedSection.FindStringSubmatch(toc[0].title)
	if first == nil || first[2] != "1" {
		return toc, bodies, err
	}
	end := 1
	for end < len(toc) {
		part := generatedSection.FindStringSubmatch(toc[end].title)
		if part == nil || part[1] != first[1] || part[2] != strconv.Itoa(end+1) {
			break
		}
		end++
	}
	if end < 2 {
		return toc, bodies, err
	}
	// 同一标题的连续网页分节可合为 TXT 中的原章节。
	merged := []chapter{{title: "内容简介"}}
	if end == len(toc) || titleKey(toc[end].title) != titleKey(first[1]) {
		merged = append(merged, chapter{title: first[1]})
	}
	merged = append(merged, toc[end:]...)
	if mergedBodies, mergeErr := splitTXT(text, bookTitle, merged); mergeErr == nil && len(mergedBodies[1]) > 0 {
		return merged, mergedBodies, nil
	}
	if end < len(toc) && titleKey(first[1]) == "前言" {
		merged := append([]chapter{{title: "内容简介"}}, toc[end:]...)
		if mergedBodies, mergeErr := splitTXT(text, bookTitle, merged); mergeErr == nil && len(mergedBodies[0]) > 0 {
			merged[0].title = first[1]
			return merged, mergedBodies, nil
		}
	}
	// 网页目录整体由分节生成时，使用 TXT 明确标记的原始章名。
	if end > len(toc)/2 {
		if chapters, nativeBodies := decoratedChapters(text); len(chapters) > 0 {
			return chapters, nativeBodies, nil
		}
	}
	if end == len(toc) {
		if chapters, nativeBodies := collectionChapters(text); len(chapters) > 0 {
			return chapters, nativeBodies, nil
		}
	}
	return toc, bodies, err
}

// Some collections separate stories with an isolated "(完)", followed by
// several blank lines and the next story's title.
func collectionChapters(text string) ([]chapter, [][]string) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n"), "\n")
	var ends []int
	intro := -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case "书籍简介", "内容简介":
			if intro == -1 {
				intro = i
			}
		case "(完)", "（完）":
			ends = append(ends, i)
		}
	}
	if intro == -1 || len(ends) < 2 || ends[len(ends)-1] != len(lines)-1 {
		return nil, nil
	}
	isTitle := func(i int) bool {
		title := strings.TrimSpace(lines[i])
		return title != "" && utf8.RuneCountInString(title) <= 24 && !strings.ContainsAny(title, "。！？!?，,：:；;“”")
	}
	first := -1
	for i := intro + 4; i < ends[0]; i++ {
		if strings.TrimSpace(lines[i-3]) == "" && strings.TrimSpace(lines[i-2]) == "" &&
			strings.TrimSpace(lines[i-1]) == "" && strings.TrimSpace(lines[i+1]) == "" && isTitle(i) {
			if first != -1 {
				return nil, nil
			}
			first = i
		}
	}
	if first == -1 {
		return nil, nil
	}
	chapters := []chapter{{title: "内容简介"}}
	bodies := [][]string{cleanLines(strings.Join(lines[:first], "\n"))}
	start := first
	for i, end := range ends {
		if start+1 >= end || !isTitle(start) {
			return nil, nil
		}
		chapters = append(chapters, chapter{title: strings.TrimSpace(lines[start])})
		bodies = append(bodies, cleanLines(strings.Join(lines[start+1:end+1], "\n")))
		if i+1 < len(ends) {
			start = end + 1
			for start < ends[i+1] && strings.TrimSpace(lines[start]) == "" {
				start++
			}
			if start-end < 4 {
				return nil, nil
			}
		}
	}
	return chapters, bodies
}

func decoratedChapters(text string) ([]chapter, [][]string) {
	var chapters []chapter
	var bodies [][]string
	title := "内容简介"
	var body []string
	for _, line := range cleanLines(text) {
		if m := decoratedHeading.FindStringSubmatch(line); m != nil {
			chapters = append(chapters, chapter{title: title})
			bodies = append(bodies, body)
			title, body = m[1], nil
		} else {
			body = append(body, line)
		}
	}
	if len(chapters) < 3 || len(body) == 0 {
		return nil, nil
	}
	return append(chapters, chapter{title: title}), append(bodies, body)
}

// 以在线目录的章名和顺序切分 TXT，无法逐一对齐时使用网页正文。
func splitTXT(text, bookTitle string, toc []chapter) ([][]string, error) {
	if len(toc) == 0 {
		return nil, fmt.Errorf("章节目录为空")
	}
	keys := make([]string, len(toc))
	for i, ch := range toc {
		keys[i] = txtTitleKey(ch.title)
	}
	bodies := make([][]string, len(toc))
	current := -1
	hasIntro := keys[0] == "内容简介"
	if hasIntro {
		// 网站会给 TXT 的书名前言单独生成一个目录项，原文件未必有同名标题。
		current = 0
	}
	for _, line := range cleanLines(text) {
		if hasIntro && current == 0 && len(bodies[0]) == 0 {
			line = trimIntroHeader(line, bookTitle)
			if line == "" {
				continue
			}
		}
		key := txtTitleKey(line)
		if current >= 0 && len(bodies[current]) == 0 && key == keys[current] {
			continue
		}
		if current+1 < len(toc) && key == keys[current+1] {
			current++
			continue
		}
		if current < 0 {
			if line == bookTitle || line == "《"+bookTitle+"》" ||
				strings.HasPrefix(line, "《"+bookTitle+"》作者：") {
				continue
			}
			return nil, fmt.Errorf("TXT 开头与目录首项《%s》不匹配", toc[0].title)
		}
		bodies[current] = append(bodies[current], line)
	}
	if current+1 != len(toc) {
		// 未找到下一章边界的那一段可能混入后续章节，只保留之前已闭合的正文。
		if current >= 0 {
			bodies[current] = nil
		}
		return bodies, fmt.Errorf("TXT 只匹配到目录第 %d/%d 章，下一项《%s》", current+1, len(toc), toc[current+1].title)
	}
	if len(bodies[current]) == 0 {
		return bodies, fmt.Errorf("TXT 末章缺少正文")
	}
	return bodies, nil
}

func trimIntroHeader(line, bookTitle string) string {
	if line == bookTitle || line == "《"+bookTitle+"》" ||
		strings.HasPrefix(line, "《"+bookTitle+"》作者：") {
		return ""
	}
	separator := strings.IndexAny(line, ":：")
	if separator < 0 {
		return line
	}
	switch strings.TrimSpace(line[:separator]) {
	case "书名", "本书名称", "作者", "本书作者":
		return ""
	case "简介", "本书简介", "内容简介":
		_, width := utf8.DecodeRuneInString(line[separator:])
		return strings.TrimSpace(line[separator+width:])
	default:
		return line
	}
}

func stripHeadings(lines []string, title string) []string {
	key := titleKey(title)
	for len(lines) > 0 {
		line := lines[0]
		if m := pageSuffix.FindStringSubmatch(line); m != nil {
			line = m[1]
		}
		if titleKey(line) != key {
			break
		}
		lines = lines[1:]
	}
	return lines
}

func titleKey(text string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.Join(strings.Fields(text), ""), ":"), "：")
}

func txtTitleKey(text string) string {
	// 网页目录会省略反斜杠，并把 TXT 中的 HTML 换行标签转成空白。
	return titleKey(strings.ReplaceAll(titleBreak.ReplaceAllString(text, ""), "\\", ""))
}

func cleanLines(text string) []string {
	text = epubexport.UnescapeHTML(text)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func textLines(root *html.Node) []string {
	return cleanLines(rawText(root))
}

func textOf(root *html.Node) string {
	return strings.TrimSpace(epubexport.UnescapeHTML(rawText(root)))
}

func rawText(root *html.Node) string {
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			out.WriteString(n.Data)
			return
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style":
				return
			case "br":
				out.WriteByte('\n')
				return
			}
		}
		block := n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div" || n.Data == "li" ||
			n.Data == "h2" || n.Data == "h3" || n.Data == "h4" || n.Data == "h5")
		if block {
			out.WriteByte('\n')
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if block {
			out.WriteByte('\n')
		}
	}
	if root != nil {
		walk(root)
	}
	return out.String()
}
