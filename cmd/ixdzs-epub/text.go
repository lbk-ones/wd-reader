package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/epubexport"
	"wd-reader/internal/resourcecatalog"
)

func (s *source) zipBodies(ctx context.Context, b book) ([][]string, error) {
	dir, err := os.MkdirTemp("", "ixdzs-epub-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(dir)
	defer os.Remove(filepath.Join(dir, "book.zip"))
	path, err := resourcecatalog.Download(ctx, s.client, resourcecatalog.File{
		URL: b.download, Name: "book.zip", ArchiveFormat: "txt", Referer: b.Source,
	}, dir, nil)
	if err != nil {
		return nil, fmt.Errorf("下载或解压 ZIP: %w", err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text, err := epubexport.DecodeTXT(data)
	if err != nil {
		return nil, fmt.Errorf("解码 TXT: %w", err)
	}
	return splitTXT(text, b.titles)
}

func splitTXT(text string, titles []string) ([][]string, error) {
	if len(titles) == 0 {
		return nil, fmt.Errorf("章节目录为空")
	}
	keys := make([]string, len(titles))
	for i, title := range titles {
		keys[i] = chapterdetect.TextKey(title)
		if keys[i] == "" {
			return nil, fmt.Errorf("目录第 %d 项缺少章名", i+1)
		}
	}
	text = epubexport.UnescapeHTML(text)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "更多电子书请访问爱下电子书") {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[len(lines)-1])
		if line != "" && line != "=已完结=" {
			break
		}
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		if strings.Trim(strings.TrimSpace(line), "-") == "章节内容开始" {
			lines = lines[i+1:]
			break
		}
	}

	bodies := make([][]string, len(titles))
	current := -1
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if current >= 0 && len(bodies[current]) == 0 && chapterdetect.TextKey(line) == keys[current] {
			continue
		}
		if current+1 < len(keys) {
			if prefix, ok := headingSuffix(line, keys[current+1]); ok {
				// ZIP 有时把卷标的处理提示残片拼在第一个真实章名前面。
				if prefix != "" && !(current < 0 && prefix == "内容还在处理中,请稍后重") {
					if current < 0 {
						return nil, fmt.Errorf("首章前有未识别的文本")
					}
					bodies[current] = append(bodies[current], prefix)
				}
				current++
				continue
			}
		}
		if current < 0 {
			if volumeLabel.MatchString(line) {
				continue
			}
			return nil, fmt.Errorf("TXT 开头与目录首项《%s》不匹配", titles[0])
		}
		bodies[current] = append(bodies[current], line)
	}
	if current+1 != len(titles) {
		return nil, fmt.Errorf("ZIP 中的 TXT 只对齐了 %d/%d 章，下一项《%s》", current+1, len(titles), titles[current+1])
	}
	return bodies, nil
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
