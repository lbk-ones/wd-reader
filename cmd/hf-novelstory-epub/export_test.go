package main

import (
	"slices"
	"testing"

	"wd-reader/internal/resourcecatalog"
)

func TestParseBookUsesDefaultsForUnstructuredTXT(t *testing.T) {
	b := resourcecatalog.Book{Title: "示例", URL: "https://example.test/book"}
	meta, titles, bodies, err := parseBook([]byte("《示例》\r\n\r\n甲 &amp;#x4E59;。\r\n后续正文。"), b)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "示例" || meta.Author != "未知作者" || meta.Category != "未分类" ||
		!slices.Equal(meta.Description, []string{"暂无简介。"}) || meta.Source != b.URL {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if !slices.Equal(titles, []string{"正文"}) || len(bodies) != 1 ||
		!slices.Equal(bodies[0], []string{"甲 乙。", "后续正文。"}) {
		t.Fatalf("unexpected chapters: %v %v", titles, bodies)
	}
}

func TestParseBookPrefersExplicitTXTMetadataAndKeepsChapterText(t *testing.T) {
	b := resourcecatalog.Book{Title: "文件名", Author: "作者提示"}
	raw := "《示例》作者: 测试作者\n分类：科幻\n简介：实际简介\n\n" +
		"第一章 开篇\n\n第一章 开篇\n\n这是第一章正文。\n\n第二章 尾声\n\n这是第二章正文。"
	meta, titles, bodies, err := parseBook([]byte(raw), b)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "示例" || meta.Author != "测试作者" || meta.Category != "科幻" ||
		!slices.Equal(meta.Description, []string{"实际简介"}) {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if !slices.Equal(titles, []string{"第一章 开篇", "第二章 尾声"}) || len(bodies) != 2 ||
		!slices.Equal(bodies[0], []string{"这是第一章正文。"}) || !slices.Equal(bodies[1], []string{"这是第二章正文。"}) {
		t.Fatalf("unexpected chapters: %v %v", titles, bodies)
	}
}
