package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/epubexport"
	"wd-reader/internal/resourcecatalog"
)

func parseBook(data []byte, b resourcecatalog.Book) (epubexport.Book, []string, [][]string, error) {
	text, err := epubexport.DecodeTXT(data)
	if err != nil {
		return epubexport.Book{}, nil, nil, fmt.Errorf("解码 TXT: %w", err)
	}
	text = epubexport.UnescapeHTML(text)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	meta, start := readMetadata(lines, b)
	lines = lines[start:]

	var titles []string
	var bodies [][]string
	title := "前言"
	detectedTitle := false
	var body []string
	flush := func() {
		if len(body) > 0 || detectedTitle {
			titles = append(titles, title)
			bodies = append(bodies, body)
			body = nil
		}
	}
	boundaries := chapterdetect.Detect(lines, nil).Boundaries
	if len(boundaries) == 0 {
		title = "正文"
	}
	next := 0
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if next < len(boundaries) && boundaries[next].Line == i {
			boundary := boundaries[next]
			next++
			if detectedTitle && len(body) == 0 && !boundary.KeepLine && boundary.Body == "" &&
				chapterdetect.TextKey(title) == chapterdetect.TextKey(boundary.Title) {
				continue
			}
			flush()
			title = boundary.Title
			detectedTitle = true
			if boundary.KeepLine {
				body = append(body, line)
			} else if boundary.Body != "" {
				body = append(body, boundary.Body)
			}
			continue
		}
		if line == "" || (len(body) == 0 && detectedTitle &&
			chapterdetect.TextKey(line) == chapterdetect.TextKey(title)) {
			continue
		}
		body = append(body, line)
	}
	flush()
	if len(bodies) == 0 {
		return epubexport.Book{}, nil, nil, fmt.Errorf("TXT 没有可导出的正文")
	}
	return meta, titles, bodies, nil
}

func readMetadata(lines []string, b resourcecatalog.Book) (epubexport.Book, int) {
	meta := epubexport.Book{Title: strings.TrimSpace(b.Title), Author: strings.TrimSpace(b.Author), Source: b.URL}
	start := 0
	for start < min(24, len(lines)) {
		line := strings.TrimSpace(lines[start])
		if line == "" || line == meta.Title || line == "《"+meta.Title+"》" {
			start++
			continue
		}
		if rest, ok := strings.CutPrefix(line, "《"); ok {
			if title, author, ok := strings.Cut(rest, "》作者"); ok &&
				(strings.HasPrefix(author, ":") || strings.HasPrefix(author, "：")) {
				line = "作者" + author
				meta.Title = strings.TrimSpace(title)
			}
		}
		index := strings.IndexAny(line, ":：")
		if index < 0 {
			break
		}
		_, width := utf8.DecodeRuneInString(line[index:])
		value := strings.TrimSpace(line[index+width:])
		switch strings.TrimSpace(line[:index]) {
		case "书名", "作品名称", "本书名称":
			if value != "" {
				meta.Title = value
			}
		case "作者", "本书作者":
			if value != "" {
				meta.Author = value
			}
		case "分类", "类型", "题材":
			meta.Category = value
		case "简介", "内容简介", "本书简介":
			if value != "" {
				meta.Description = append(meta.Description, value)
			}
		default:
			return withDefaults(meta), start
		}
		start++
	}
	return withDefaults(meta), start
}

func withDefaults(meta epubexport.Book) epubexport.Book {
	if meta.Title == "" {
		meta.Title = "未命名"
	}
	if meta.Author == "" {
		meta.Author = "未知作者"
	}
	if meta.Category == "" {
		meta.Category = "未分类"
	}
	if len(meta.Description) == 0 {
		meta.Description = []string{"暂无简介。"}
	}
	return meta
}
