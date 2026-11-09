package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nwaples/rardecode/v2"
	"golang.org/x/net/html"

	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/epubexport"
	"wd-reader/internal/resourcecatalog"
)

func (s *source) downloadTXT(ctx context.Context, b book) (string, error) {
	if b.download == "" {
		return "", fmt.Errorf("书籍没有 TXT 下载页")
	}
	pageURL, err := s.base.Parse(b.download)
	if err != nil {
		return "", err
	}
	doc, err := s.page(ctx, pageURL.String())
	if err != nil {
		return "", err
	}
	files := [2]resourcecatalog.File{
		{Name: "book.txt", Referer: pageURL.String()},
		{Name: "book.zip", ArchiveFormat: "txt", Referer: pageURL.String()},
	}
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || n.Data != "a" {
			continue
		}
		u, err := pageURL.Parse(strings.TrimSpace(attr(n, "href")))
		if err != nil || !s.allowed(u) || (u.Host != s.base.Host && u.Host != "jjxswshuku.com") ||
			u.Path != "/e/DownSys/doaction.php" {
			continue
		}
		query := u.Query()
		if query.Get("id") != b.id {
			continue
		}
		switch query.Get("pathid") {
		case "0":
			if files[0].URL == "" {
				files[0].URL = u.String()
			}
		case "1":
			if files[1].URL == "" {
				files[1].URL = u.String()
			}
		}
	}
	var downloadErr error
	for _, file := range files {
		if file.URL == "" {
			continue
		}
		text, err := s.downloadTextFile(ctx, file)
		if err == nil || errors.Is(err, errRateLimited) || ctx.Err() != nil {
			return text, err
		}
		downloadErr = errors.Join(downloadErr, fmt.Errorf("%s: %w", file.Name, err))
	}
	if downloadErr == nil {
		downloadErr = fmt.Errorf("下载页没有可用的整本 TXT 或 ZIP 链接")
	}
	return "", downloadErr
}

func (s *source) downloadTextFile(ctx context.Context, file resourcecatalog.File) (string, error) {
	dir, err := os.MkdirTemp("", "jjxs-epub-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dir)
	defer os.Remove(filepath.Join(dir, file.Name))
	if err := s.wait(ctx); err != nil {
		return "", err
	}
	path, err := resourcecatalog.Download(ctx, s.client, file, dir, nil)
	if err != nil {
		var status *resourcecatalog.DownloadStatusError
		if errors.As(err, &status) && status.StatusCode == http.StatusTooManyRequests {
			return "", s.rateLimited("/e/DownSys/doaction.php", status.RetryAfter)
		}
		return "", err
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.HasPrefix(data, []byte("Rar!\x1a\x07")) {
		data, err = readRARTXT(data)
		if err != nil {
			return "", fmt.Errorf("读取 RAR 内的 TXT: %w", err)
		}
	}
	text, err := epubexport.DecodeTXT(data)
	if err == nil && (strings.TrimSpace(text) == "" || strings.ContainsRune(text, 0)) {
		return "", fmt.Errorf("下载内容为空或不是 TXT")
	}
	return text, err
}

func readRARTXT(data []byte) ([]byte, error) {
	const limit = resourcecatalog.MaxDownloadBytes
	r, err := rardecode.NewReader(bytes.NewReader(data), rardecode.MaxDictionarySize(limit))
	if err != nil {
		return nil, err
	}
	var text []byte
	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if !entry.Mode().IsRegular() || !strings.EqualFold(path.Ext(entry.Name), ".txt") {
			continue
		}
		if text != nil {
			return nil, fmt.Errorf("RAR 包含多个 TXT 文件")
		}
		if entry.UnPackedSize > limit {
			return nil, fmt.Errorf("TXT 超过 100 MiB 解压限制")
		}
		text, err = io.ReadAll(io.LimitReader(r, limit+1))
		if err != nil {
			return nil, err
		}
		if int64(len(text)) > limit {
			return nil, fmt.Errorf("TXT 超过 100 MiB 解压限制")
		}
	}
	if len(text) == 0 {
		return nil, fmt.Errorf("RAR 缺少可用的 TXT 文件")
	}
	return text, nil
}

var (
	generatedSection = regexp.MustCompile(`^(.+?)\s+第([1-9][0-9]*)节$`)
	decoratedHeading = regexp.MustCompile(`^☆\.\s*(第[0-9０-９]+章\s*[:：]\s*\S.*)$`)
	titleBreak       = regexp.MustCompile(`(?i)<br\s*/?>`)
)

func prepareTXT(text, bookTitle string, toc []chapter) ([]chapter, [][]string, error) {
	if len(toc) == 0 {
		return wholeTXT(text, bookTitle)
	}
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
		// 书名分节是网站按长度生成的，原文未必有章名或分节边界。
		if titleKey(first[1]) == titleKey(bookTitle) {
			return wholeTXT(text, bookTitle)
		}
	}
	return toc, bodies, err
}

func wholeTXT(text, bookTitle string) ([]chapter, [][]string, error) {
	if chapters, bodies := decoratedChapters(text); len(chapters) > 0 {
		return chapters, bodies, nil
	}
	if chapters, bodies := collectionChapters(text); len(chapters) > 0 {
		return chapters, bodies, nil
	}
	lines := physicalLines(text)
	boundaries := chapterdetect.Detect(lines, nil).Boundaries
	if len(boundaries) == 0 {
		body := nonemptyLines(lines)
		if len(body) == 0 {
			return nil, nil, fmt.Errorf("TXT 正文为空")
		}
		return []chapter{{title: bookTitle}}, [][]string{body}, nil
	}
	var chapters []chapter
	var bodies [][]string
	if preface := nonemptyLines(lines[:boundaries[0].Line]); len(preface) > 0 {
		chapters = append(chapters, chapter{title: "前言"})
		bodies = append(bodies, preface)
	}
	for i, boundary := range boundaries {
		end := len(lines)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].Line
		}
		start := boundary.Line
		if !boundary.KeepLine {
			start++
		}
		body := nonemptyLines(lines[start:end])
		if boundary.Body != "" && !boundary.KeepLine {
			body = append([]string{boundary.Body}, body...)
		}
		chapters = append(chapters, chapter{title: boundary.Title})
		bodies = append(bodies, body)
	}
	return chapters, bodies, nil
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

// 以在线目录的章名和顺序切分 TXT。
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
	for i, body := range bodies {
		if len(body) == 0 && !(i == 0 && hasIntro) {
			return bodies, fmt.Errorf("TXT 第 %d 章《%s》缺少正文", i+1, toc[i].title)
		}
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
		label, _, _ := chapterPageTitle(line)
		if titleKey(line) != key && titleKey(label) != key {
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
	return nonemptyLines(physicalLines(text))
}

func physicalLines(text string) []string {
	text = epubexport.UnescapeHTML(text)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return strings.Split(text, "\n")
}

func nonemptyLines(lines []string) []string {
	var cleaned []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return cleaned
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
