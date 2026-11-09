package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/epubexport"
	"wd-reader/internal/resourcecatalog"
)

func (s *source) zipText(ctx context.Context, b book) (string, error) {
	dir, err := os.MkdirTemp("", "ixdzs-epub-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dir)
	defer os.Remove(filepath.Join(dir, "book.zip"))
	path, err := resourcecatalog.Download(ctx, s.client, resourcecatalog.File{
		URL: b.download, Name: "book.zip", ArchiveFormat: "txt", Referer: b.Source,
	}, dir, nil)
	if err != nil {
		return "", fmt.Errorf("下载或解压 ZIP: %w", err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text, err := epubexport.DecodeTXT(data)
	if err != nil {
		return "", fmt.Errorf("解码 TXT: %w", err)
	}
	return text, nil
}

const processingNotice = "内容还在处理中,请稍后重"

var numberedHeading = regexp.MustCompile("^第?([0-9０-９零〇一二三四五六七八九十百千万亿两]+)([章节回【.．、])")

type headingNumber struct {
	number int
	unit   string
}

func chapterNumber(key string) (headingNumber, string) {
	match := numberedHeading.FindStringSubmatch(key)
	if match == nil {
		return headingNumber{}, ""
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + r - '０'
		}
		return r
	}, match[1])
	unit, end := match[2], len(match[0])
	if unit == "【" {
		unit, end = "章", end-len("【")
	} else if strings.Contains(".．、", unit) {
		unit = "章"
	}
	return headingNumber{chapterdetect.Ordinal(digits), unit}, strings.TrimLeft(key[end:], ":：、")
}

func sameChapterTitle(left, right string) bool {
	left, right = chapterdetect.TextKey(left), chapterdetect.TextKey(right)
	if left == right {
		return left != ""
	}
	number, leftTitle := chapterNumber(left)
	other, rightTitle := chapterNumber(right)
	if number.unit == "" || number.number < 0 || number != other {
		return false
	}
	if leftTitle == rightTitle {
		return true
	}
	if len(leftTitle) > len(rightTitle) {
		leftTitle, rightTitle = rightTitle, leftTitle
	}
	suffix, ok := strings.CutPrefix(rightTitle, leftTitle)
	if !ok || leftTitle == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(suffix)
	return strings.ContainsRune("（(【[", first) ||
		(utf8.RuneCountInString(leftTitle) >= 2 && utf8.RuneCountInString(suffix) <= 2)
}

func splitTXT(text string, b book) ([][]string, error) {
	titles := b.titles
	if len(titles) == 0 {
		return nil, fmt.Errorf("章节目录为空")
	}
	keys := make([]string, len(titles))
	byTitle := make(map[string][]int)
	byNumber := make(map[headingNumber][]int)
	for i, title := range titles {
		keys[i] = chapterdetect.TextKey(title)
		if keys[i] == "" {
			return nil, fmt.Errorf("目录第 %d 项缺少章名", i+1)
		}
		byTitle[keys[i]] = append(byTitle[keys[i]], i)
		if number, _ := chapterNumber(keys[i]); number.unit != "" {
			byNumber[number] = append(byNumber[number], i)
		}
	}
	lines, _, _ := zipTextLines(text)
	bodies := make([][]string, len(titles))
	current := -1
	firstGap := -1
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == processingNotice {
			continue
		}
		key := chapterdetect.TextKey(strings.TrimPrefix(line, processingNotice))
		if current >= 0 && len(bodies[current]) == 0 && key == keys[current] &&
			(current+1 == len(keys) || key != keys[current+1]) {
			continue
		}
		next, prefix := -1, ""
		for i := current + 1; i < len(keys); i++ {
			before, exact := headingSuffix(line, keys[i])
			if exact && (current >= 0 || before == "" || before == processingNotice) {
				next, prefix = i, before
				break
			}
			if sameChapterTitle(key, keys[i]) {
				next = i
				break
			}
			if b.paths[i] != "" || !volumeLabel.MatchString(titles[i]) {
				break
			}
		}
		if next == -1 {
			next = laterHeading(key, current, keys, byTitle, byNumber)
		}
		if next == -1 {
			if current >= 0 {
				bodies[current] = append(bodies[current], line)
			}
			continue
		}
		for i := current + 1; i < next; i++ {
			if b.paths[i] == "" && volumeLabel.MatchString(titles[i]) {
				continue
			}
			// A skipped boundary leaves the preceding chapter's end uncertain.
			if firstGap == -1 {
				firstGap = i
			}
			if current >= 0 {
				bodies[current] = nil
			}
			prefix = ""
			break
		}
		if current >= 0 && prefix != "" && prefix != processingNotice {
			bodies[current] = append(bodies[current], prefix)
		}
		current = next
	}
	if current+1 != len(titles) {
		if current >= 0 {
			bodies[current] = nil
		}
		if firstGap == -1 {
			firstGap = current + 1
		}
	}
	if firstGap >= 0 {
		return bodies, fmt.Errorf("ZIP 无法确认目录第 %d/%d 项《%s》的正文边界", firstGap+1, len(titles), titles[firstGap])
	}
	return bodies, nil
}

func zipTextLines(text string) (lines []string, hasStart, hasEnd bool) {
	text = epubexport.UnescapeHTML(text)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines = strings.Split(strings.TrimSpace(text), "\n")
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[len(lines)-1])
		if line == "=已完结=" {
			hasEnd = true
		} else if line != "" && !strings.HasPrefix(line, "更多电子书请访问爱下电子书") {
			break
		}
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		if strings.Trim(strings.TrimSpace(line), "-") == "章节内容开始" {
			return lines[i+1:], true, hasEnd
		}
	}
	return lines, false, hasEnd
}

// A complete TXT edition can have more chapters or a different numbering than
// the website. Use its own boundaries only when it has a full set of bodies.
func nativeTXT(text string, b book) ([]string, [][]string) {
	lines, hasStart, hasEnd := zipTextLines(text)
	if !hasStart || !hasEnd || strings.Contains(text, processingNotice) {
		return nil, nil
	}
	required := 0
	for _, path := range b.paths {
		if path != "" {
			required++
		}
	}
	boundaries := chapterdetect.Detect(lines, nil).Boundaries
	// ZIP 原始章名顶格写入，缩进中的二次章名和普通段落仍属于正文。
	boundaries = slices.DeleteFunc(boundaries, func(boundary chapterdetect.Boundary) bool {
		line := lines[boundary.Line]
		return strings.TrimLeftFunc(line, unicode.IsSpace) != line
	})
	if len(boundaries) < max(3, required) {
		return nil, nil
	}
	first, _ := chapterNumber(chapterdetect.TextKey(boundaries[0].Title))
	if first.number != 1 {
		return nil, nil
	}
	var titles []string
	var bodies [][]string
	for i, boundary := range boundaries {
		end := len(lines)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].Line
		}
		start := boundary.Line
		if !boundary.KeepLine {
			start++
		}
		var body []string
		if boundary.Body != "" {
			body = append(body, boundary.Body)
		}
		for _, line := range lines[start:end] {
			if line = strings.TrimSpace(line); line != "" {
				body = append(body, line)
			}
		}
		if len(body) == 0 {
			return nil, nil
		}
		if i == 0 {
			var opening []string
			for _, line := range lines[:boundary.Line] {
				if line = strings.TrimSpace(line); line != "" {
					opening = append(opening, line)
				}
			}
			body = append(opening, body...)
		}
		titles = append(titles, boundary.Title)
		bodies = append(bodies, body)
	}
	return titles, bodies
}

func laterHeading(key string, current int, keys []string, byTitle map[string][]int, byNumber map[headingNumber][]int) int {
	exact := byTitle[key]
	if i := sort.SearchInts(exact, current+1); i < len(exact) {
		return exact[i]
	}
	number, _ := chapterNumber(key)
	if number.unit != "" {
		numbered := byNumber[number]
		for _, i := range numbered[sort.SearchInts(numbered, current+1):] {
			if sameChapterTitle(key, keys[i]) {
				return i
			}
		}
	}
	return -1
}

// 章名有时紧贴上一章末段。匹配行尾时忽略空白，但把章名前的原文字节留给上一章。
func headingSuffix(line, key string) (string, bool) {
	for len(key) > 0 {
		if len(line) == 0 {
			return "", false
		}
		got, width := utf8.DecodeLastRuneInString(line)
		line = line[:len(line)-width]
		if unicode.IsSpace(got) || got == '\u200b' || got == '\ufeff' {
			continue
		}
		want, width := utf8.DecodeLastRuneInString(key)
		if got != want {
			return "", false
		}
		key = key[:len(key)-width]
	}
	return strings.TrimSpace(line), true
}
