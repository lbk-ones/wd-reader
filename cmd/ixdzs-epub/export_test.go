package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"wd-reader/internal/epubexport"
)

func TestSplitZIPTextPreservesParagraphBeforeJoinedHeading(t *testing.T) {
	titles := []string{"第1章 开篇", "第2章 末章 & 短语", "番外"}
	text := strings.Join([]string{
		"『内容简介: 第1章 开篇』",
		"------章节内容开始-------",
		"第一卷",
		"　　内容还在处理中,请稍后重第1章开篇",
		"　　第1章 开篇",
		"　　甲&amp;#x4E59;",
		"　　上一章的末句。第2章末章&nbsp;&amp; 短语",
		"　　第二章内容。",
		"第二章末句。番外",
		"　　番外正文。",
		"=已完结=",
		"更多电子书请访问爱下电子书,简体:https://ixdzs8.com",
	}, "\r\n")
	data, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := epubexport.DecodeTXT(data)
	if err != nil {
		t.Fatal(err)
	}
	bodies, err := splitTXT(decoded, titles)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"甲乙", "上一章的末句。"}, {"第二章内容。", "第二章末句。"}, {"番外正文。"}}
	for i, paragraphs := range bodies {
		if !slices.Equal(paragraphs, want[i]) {
			t.Fatalf("chapter %d = %v, want %v", i+1, paragraphs, want[i])
		}
	}
}

func TestCatalogueDistinguishesUnlinkedChaptersAndVolumeSlots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<meta property="og:novel:status" content="已完结">
			<meta property="og:novel:book_name" content="书名">
			<meta property="og:novel:author" content="作者">
			<meta property="og:novel:category" content="玄幻">
			<meta property="og:image" content="/cover.jpg">
			<meta property="og:novel:latest_chapter_url" content="/read/7/p4.html">
			<a href="https://down7.ixdzs8.com/7.zip">TXT 下载</a><p id="intro">简介</p>
			<span class="sub-text-r">共5章</span><div id="a-list"><ul class="u-chapter">
			<li>第一卷</li><li>第1章</li><li><a href="/read/7/p3.html">开篇</a></li>
			<li><a href="/read/7/p4.html">完结</a></li>
			<li><a href="/read/7/p4.html">完结</a></li></ul></div>`)
	}))
	defer server.Close()
	src, stop := newSource(0)
	defer stop()
	src.base, _ = url.Parse(server.URL)
	b, err := src.details(context.Background(), "7")
	if err != nil || !slices.Equal(b.titles, []string{"第1章", "开篇", "完结", "完结"}) {
		t.Fatalf("unexpected catalogue: %v, %v", b.titles, err)
	}
	bodies, err := splitTXT("第1章\n第一章正文\n开篇\n开篇正文\n完结\n完结正文\n完结\n完结正文", b.titles)
	if err != nil {
		t.Fatal(err)
	}
	titles, paths, uniqueBodies, err := uniqueChapters(b, bodies)
	if err != nil || !slices.Equal(titles, []string{"第1章", "开篇", "完结"}) || paths[0] != "" ||
		!slices.Equal(uniqueBodies[0], []string{"第一章正文"}) {
		t.Fatalf("unexpected deduplicated chapters: %v, %v", titles, err)
	}
}

func TestSplitZIPTextRejectsIncompleteCatalogue(t *testing.T) {
	text := "第1章 开篇\n正文。\n=已完结="
	if _, err := splitTXT(text, []string{"第1章 开篇", "第2章 完结"}); err == nil {
		t.Fatal("accepted a ZIP text without the final chapter")
	}
}
