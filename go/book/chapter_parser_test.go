package book

import (
	"bufio"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
	"wd-reader/go/constant"
)

func TestParseReadableChapters(t *testing.T) {
	tests := []struct {
		name, text, splitType, splitValue string
		want                              []readableChapter
	}{
		{
			name: "English sentences are not headings", splitType: "1",
			text: "Chapter One\nThe story begins here.\nShe opened the door.\nCHAPTER II\nHe said: hello.\n",
			want: []readableChapter{{"Chapter One", []string{"The story begins here.", "She opened the door."}}, {"CHAPTER II", []string{"He said: hello."}}},
		},
		{
			name: "English prose without headings is retained", splitType: "1",
			text: "The story begins here.\nShe opened the door.",
			want: []readableChapter{{"正文", []string{"The story begins here.", "She opened the door."}}},
		},
		{
			name: "TOC entries do not shadow real chapters", splitType: "1",
			text: "目录\n第一章 开始\n第二章 后续\n第一章 开始\n正文内容。\n第二章 后续\n后续内容。\n第三章 空章\n",
			want: []readableChapter{{"第一章 开始", []string{"正文内容。"}}, {"第二章 后续", []string{"后续内容。"}}},
		},
		{
			name: "multiple repeated headings and Unicode whitespace", splitType: "1",
			text: "\ufeff第一章 开始\n第一章　开始\n第一章 开始\n\u200b\n正文。\n",
			want: []readableChapter{{"第一章 开始", []string{"正文。"}}},
		},
		{
			name: "duplicate title and body omitted but different bodies preserved", splitType: "1",
			text: "第一章 开始\n正文。\n第二章 后续\n后续。\n第一章 开始\n正 文。\n第一章 开始\n另一段正文。",
			want: []readableChapter{{"第一章 开始", []string{"正文。"}}, {"第二章 后续", []string{"后续。"}}, {"第一章 开始（同名 2）", []string{"另一段正文。"}}},
		},
		{
			name: "short chapters retained", splitType: "1",
			text: "第一章\n完。\n第二章\n嗯。",
			want: []readableChapter{{"第一章", []string{"完。"}}, {"第二章", []string{"嗯。"}}},
		},
		{
			name: "preface and unchaptered text preserved", splitType: "1",
			text: "作者前言。\n第一章\n故事。",
			want: []readableChapter{{"正文", []string{"作者前言。"}}, {"第一章", []string{"故事。"}}},
		},
		{
			name: "no headings", splitType: "1", text: "一段正文。\n另一段正文。",
			want: []readableChapter{{"正文", []string{"一段正文。", "另一段正文。"}}},
		},
		{
			name: "paragraph boundary does not create empty trailing chapter", splitType: "2", splitValue: "2",
			text: "甲\n\n乙\n丙\n丁\n",
			want: []readableChapter{{"0、", []string{"甲", "乙"}}, {"1、", []string{"丙", "丁"}}},
		},
		{
			name: "paragraph remainder retained", splitType: "2", splitValue: "2", text: "甲\n乙\n丙",
			want: []readableChapter{{"0、", []string{"甲", "乙"}}, {"1、", []string{"丙"}}},
		},
		{
			name: "custom headings", splitType: "3", splitValue: "^章节", text: "章节甲\n章节乙\n正文。",
			want: []readableChapter{{"章节乙", []string{"正文。"}}},
		},
		{
			name: "chapter and volume regex preset", splitType: "3",
			splitValue: "^第[ 　]*[0-9零〇一二三四五六七八九十百千万两壹贰叁肆伍陆柒捌玖拾佰仟]+[ 　]*[章卷]",
			text:       "第一卷 开篇\n卷的介绍。\n第 2 章 故事\n正文。",
			want:       []readableChapter{{"第一卷 开篇", []string{"卷的介绍。"}}, {"第 2 章 故事", []string{"正文。"}}},
		},
		{
			name: "empty headings only", splitType: "1", text: "第一章\n第二章\n\n", want: []readableChapter{},
		},
		{
			name: "empty file", splitType: "1", text: "", want: []readableChapter{},
		},
		{
			name: "zero paragraph count uses smart split", splitType: "2", splitValue: "0", text: "第一章\n正文。",
			want: []readableChapter{{"第一章", []string{"正文。"}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseReadableChapters(bufio.NewScanner(strings.NewReader(test.text)), test.splitType, test.splitValue)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestUTF16BookChapters(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	bookDir := ParsePath()
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, endian := range []string{"LE", "BE"} {
		t.Run(endian, func(t *testing.T) {
			var order binary.ByteOrder = binary.LittleEndian
			bom := []byte{0xff, 0xfe}
			if endian == "BE" {
				order = binary.BigEndian
				bom = []byte{0xfe, 0xff}
			}
			text := "第一章 开始\r\n这是正文，包含表情😀。\r\n第二章 后续\r\n后续内容。"
			data := append([]byte{}, bom...)
			for _, unit := range utf16.Encode([]rune(text)) {
				var pair [2]byte
				order.PutUint16(pair[:], unit)
				data = append(data, pair[:]...)
			}
			fileName := "utf16" + endian + ".txt"
			if err := os.WriteFile(filepath.Join(bookDir, fileName), data, 0600); err != nil {
				t.Fatal(err)
			}
			if got := GetChapterListByFileNameExtract(fileName, "1", ""); got != "第一章 开始\n第二章 后续" {
				t.Fatalf("directory decoded incorrectly: %q", got)
			}
			if got := GetChapterContentByChpaterNameExtract(fileName, "第一章 开始", "1", ""); got != "这是正文，包含表情😀。" {
				t.Fatalf("body decoded incorrectly: %q", got)
			}
		})
	}
}

func TestParseReadableChaptersErrors(t *testing.T) {
	for _, config := range [][2]string{{"2", "-1"}, {"2", "invalid"}, {"3", "["}} {
		if _, err := parseReadableChapters(bufio.NewScanner(strings.NewReader("正文")), config[0], config[1]); err == nil {
			t.Fatalf("expected error for %v", config)
		}
	}
	scanner := bufio.NewScanner(strings.NewReader(strings.Repeat("x", 100)))
	scanner.Buffer(make([]byte, 8), 16)
	if _, err := parseReadableChapters(scanner, "1", ""); err == nil {
		t.Fatal("scanner errors must be surfaced instead of returning a truncated book")
	}
}

func TestChapterDirectoryAndContentAgree(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	bookDir := ParsePath()
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatal(err)
	}
	text := "\ufeff目录\n第一章\n第二章\n第一章\n真正的正文。\n第二章\n后续正文。\n第一章\n另一份正文。\n第三章\n"
	if err := os.WriteFile(filepath.Join(bookDir, "test.txt"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if got := GetChapterListByFileNameExtract("test.txt", "1", ""); got != "第一章\n第二章\n第一章（同名 2）" {
		t.Fatalf("unexpected directory: %q", got)
	}
	for name, body := range map[string]string{"第一章": "真正的正文。", "第二章": "后续正文。", "第一章（同名 2）": "另一份正文。"} {
		if got := GetChapterContentByChpaterNameExtract("test.txt", name, "1", ""); got != body {
			t.Fatalf("chapter %q: got %q, want %q", name, got, body)
		}
	}
	if got := GetChapterContentByChpaterNameExtract("test.txt", "第三章", "1", ""); !strings.HasPrefix(got, constant.ERROR_PREFIX) {
		t.Fatalf("expected missing chapter error, got %q", got)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "empty.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got := GetChapterListByFileNameExtract("empty.txt", "1", ""); got != "" {
		t.Fatalf("empty file: got %q", got)
	}
	longBody := strings.Repeat("很长的正文。", 12000)
	if err := os.WriteFile(filepath.Join(bookDir, "long.txt"), []byte("\ufeff第一章\n"+longBody), 0600); err != nil {
		t.Fatal(err)
	}
	if got := GetChapterContentByChpaterNameExtract("long.txt", "第一章", "1", ""); got != longBody {
		t.Fatal("long paragraph was truncated or failed to load")
	}
}
