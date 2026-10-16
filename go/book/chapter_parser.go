package book

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"wd-reader/go/constant"
)

type readableChapter struct {
	Name string
	Body []string
}

func chapterTextKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff' {
			return -1
		}
		return r
	}, text)
}

// Both the directory and content lookup use the same chapters, so an empty
// occurrence in an embedded table of contents cannot shadow the real chapter.
func parseReadableChapters(scanner *bufio.Scanner, splitType, splitValue string) ([]readableChapter, error) {
	chapterPattern := constant.RegChapter
	linesPerChapter := 0
	if splitType == "2" && splitValue != "0" {
		var err error
		linesPerChapter, err = strconv.Atoi(splitValue)
		if err != nil || linesPerChapter <= 0 {
			return nil, fmt.Errorf("每章段落数必须为正整数")
		}
	} else if splitType == "3" && splitValue != "" {
		var err error
		chapterPattern, err = regexp.Compile(splitValue)
		if err != nil {
			return nil, fmt.Errorf("断章正则无效: %w", err)
		}
	}

	chapters := make([]readableChapter, 0)
	seen := make(map[string]map[[32]byte]bool)
	usedNames := make(map[string]bool)
	var name string
	var body []string
	flush := func() {
		if len(body) == 0 {
			return
		}
		baseName := name
		if baseName == "" {
			// A standalone TOC label is not a preface or readable chapter.
			label := strings.ToLower(chapterTextKey(strings.Join(body, "")))
			if label == "目录" || label == "contents" || label == "tableofcontents" {
				body = nil
				return
			}
			baseName = "正文"
		}
		key := chapterTextKey(baseName)
		digest := sha256.Sum256([]byte(chapterTextKey(strings.Join(body, ""))))
		if seen[key] == nil {
			seen[key] = make(map[[32]byte]bool)
		}
		if !seen[key][digest] {
			seen[key][digest] = true
			uniqueName := baseName
			for n := 2; usedNames[chapterTextKey(uniqueName)]; n++ {
				uniqueName = fmt.Sprintf("%s（同名 %d）", baseName, n)
			}
			usedNames[chapterTextKey(uniqueName)] = true
			chapters = append(chapters, readableChapter{Name: uniqueName, Body: body})
		}
		body = nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if chapterTextKey(line) == "" || strings.HasPrefix(line, "<img") || strings.HasPrefix(line, "<script") || strings.HasPrefix(line, "<style") {
			continue
		}
		if linesPerChapter > 0 {
			if len(body) == 0 {
				name = fmt.Sprintf("%d、", len(chapters))
			}
			body = append(body, line)
			if len(body) == linesPerChapter {
				flush()
			}
			continue
		}
		if chapterPattern.MatchString(line) {
			// Any number of repeated heading lines before the body is harmless.
			if len(body) == 0 && chapterTextKey(name) == chapterTextKey(line) {
				continue
			}
			flush()
			name = line
			continue
		}
		body = append(body, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取书籍失败: %w", err)
	}
	flush()
	return chapters, nil
}

func readBookChapters(fileName, splitType, splitValue string) ([]readableChapter, error) {
	path := filepath.Join(GetAppPath(), constant.BOOK_PATH, fileName)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := GetScanner(path, file)
	// Converted books can contain paragraphs longer than Scanner's 64 KiB default.
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	return parseReadableChapters(scanner, splitType, splitValue)
}
