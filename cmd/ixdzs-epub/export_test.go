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
	titles := []string{"第一卷 起卷", "第1章 开篇", "第2章 末章 & 短语", "番外"}
	text := strings.Join([]string{
		"『内容简介: 第1章 开篇』",
		"------章节内容开始-------",
		"第一卷起卷",
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
	bodies, err := splitTXT(decoded, book{titles: titles, paths: []string{"", "p2", "p3", "p4"}})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{nil, {"甲乙", "上一章的末句。"}, {"第二章内容。", "第二章末句。"}, {"番外正文。"}}
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
			<meta property="og:novel:latest_chapter_url" content="/read/7/p40.html">
			<a href="https://down7.ixdzs8.com/7.zip">TXT 下载</a><p id="intro">简介</p>
			<span class="sub-text-r">共8章</span><div id="a-list"><ul class="u-chapter">
			<li>第一卷</li><li>第1章</li><li><a href="/read/7/p30.html">开篇</a></li>
			<li><a href="/read/7/p40.html">完结</a></li>
			<li><a href="/read/7/p40.html">完结</a></li>
			<li><a href="/read/7/p40.html">番外</a></li><li>附录</li><li>后记</li></ul></div>`)
	}))
	defer server.Close()
	src, stop := newSource(0)
	defer stop()
	src.base, _ = url.Parse(server.URL)
	b, err := src.details(context.Background(), "7")
	if err != nil || !slices.Equal(b.titles, []string{"第一卷", "第1章", "开篇", "完结", "完结", "番外", "附录", "后记"}) {
		t.Fatalf("unexpected catalogue: %v, %v", b.titles, err)
	}
	bodies, err := splitTXT("第1章\n第一章正文\n开篇\n开篇正文\n完结\n完结正文\n完结\n完结正文\n番外\n番外正文\n附录\n"+processingNotice+"后记\n后记正文", b)
	if err != nil {
		t.Fatal(err)
	}
	titles, paths, uniqueBodies := uniqueChapters(b, bodies)
	if !slices.Equal(titles, []string{"第1章", "开篇", "完结", "番外", "后记"}) || paths[0] != "" ||
		!slices.Equal(uniqueBodies[0], []string{"第一章正文"}) || !slices.Equal(uniqueBodies[3], []string{"番外正文"}) ||
		!slices.Equal(uniqueBodies[4], []string{"后记正文"}) {
		t.Fatalf("unexpected deduplicated chapters: %v", titles)
	}
}

func TestSharedChapterLinkPreservesDistinctTXT(t *testing.T) {
	b := book{titles: []string{"一章", "一章", "一章"}, paths: []string{"p1", "p1", "p1"}}
	_, _, bodies := uniqueChapters(b, [][]string{{"甲"}, {"乙"}, {"乙"}})
	if len(bodies) != 2 || !slices.Equal(bodies[0], []string{"甲"}) || !slices.Equal(bodies[1], []string{"乙"}) {
		t.Fatalf("unexpected bodies: %v", bodies)
	}
}

func TestDetailsLoadsFullCatalogue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/read/7/":
			fmt.Fprint(w, `<meta property="og:novel:status" content="已完结">
				<meta property="og:novel:book_name" content="书名">
				<meta property="og:novel:author" content="作者">
				<meta property="og:novel:category" content="玄幻">
				<meta property="og:image" content="/cover.jpg">
				<meta property="og:novel:latest_chapter_url" content="/read/7/p3.html">
				<a href="https://down7.ixdzs8.com/7.zip">TXT 下载</a><p id="intro">简介</p>
				<span class="sub-text-r">共3章</span>
				<ul class="u-chapter cfirst"><li class="catalog-all"><a href="#a-list">查看完整章节目录</a></li></ul>
				<div id="a-list"><ul class="u-chapter">
				<li><a href="/read/7/p1.html">开篇</a></li>
				<li><a href="/read/7/p3.html">完结</a></li></ul></div>`)
		case "/novel/html":
			if r.Method != http.MethodPost || r.PostFormValue("bid") != "7" ||
				r.Header.Get("X-Requested-With") != "XMLHttpRequest" || !strings.HasSuffix(r.Referer(), "/read/7/") {
				http.Error(w, "unexpected catalogue request", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `<li><a href="/read/7/p1.html">开篇</a></li>
				<li><a href="/read/7/p2.html">中篇</a></li>
				<li><a href="/read/7/p3.html">完结</a></li>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	src, stop := newSource(0)
	defer stop()
	src.base, _ = url.Parse(server.URL)
	b, err := src.details(context.Background(), "7")
	if err != nil || !slices.Equal(b.titles, []string{"开篇", "中篇", "完结"}) ||
		!slices.Equal(b.paths, []string{"/read/7/p1.html", "/read/7/p2.html", "/read/7/p3.html"}) {
		t.Fatalf("unexpected catalogue: %v, %v, %v", b.titles, b.paths, err)
	}
}

func TestSplitZIPTextRejectsIncompleteCatalogue(t *testing.T) {
	text := "第1章 开篇\n正文。\n=已完结="
	b := book{titles: []string{"第1章 开篇", "第2章 完结"}, paths: []string{"p1", "p2"}}
	if _, err := splitTXT(text, b); err == nil {
		t.Fatal("accepted a ZIP text without the final chapter")
	}
}

func TestSplitZIPTextResumesAfterUnmatchedChapter(t *testing.T) {
	b := book{
		titles: []string{"第1章 开篇（求票）", "2【中篇】", "第3章 缺失", "004.魔神意", "第5章 尾声"},
		paths:  []string{"p1", "p2", "p3", "p4", "p5"},
	}
	text := "第一章开篇\n甲\n第2章【中篇】\n乙\n第三章 改名\n丙\n第4章魔神意图\n丁\n第5章尾声\n戊"
	bodies, err := splitTXT(text, b)
	if err == nil || len(bodies) != 5 || len(bodies[1]) != 0 || len(bodies[2]) != 0 ||
		!slices.Equal(bodies[0], []string{"甲"}) || !slices.Equal(bodies[3], []string{"丁"}) ||
		!slices.Equal(bodies[4], []string{"戊"}) {
		t.Fatalf("unexpected recovered chapters: %v, %v", bodies, err)
	}
}

func TestNativeTXTUsesOwnChapterOrderWhenEditionsDiffer(t *testing.T) {
	b := book{paths: []string{"p1", "p2", "p3"}}
	text := "------章节内容开始-------\n\n第1章 相逢\n\n　　相逢正文。\n\n第2章 插曲\n\n　　插曲正文。\n\n第3章 尾声\n\n　　尾声正文。\n\n=已完结="
	titles, bodies := nativeTXT(text, b)
	if !slices.Equal(titles, []string{"第1章 相逢", "第2章 插曲", "第3章 尾声"}) ||
		len(bodies) != 3 || !slices.Equal(bodies[1], []string{"插曲正文。"}) {
		t.Fatalf("native TXT = %v, %v", titles, bodies)
	}
	if titles, _ := nativeTXT(strings.TrimSuffix(text, "=已完结="), b); titles != nil {
		t.Fatalf("accepted a TXT without the completion marker: %v", titles)
	}
}
