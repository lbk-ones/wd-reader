package source

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wd-reader/internal/chapterdetect"
)

// writeEPUB 在测试临时目录创建最小电子书，并补充标准容器声明供格式解析器定位内容清单。
// files 使用压缩包内路径作为键；测试不能依赖这些文件在压缩包中的写入顺序。
func writeEPUB(t *testing.T, files map[string]string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "book.epub")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	files["META-INF/container.xml"] = `<container><rootfiles><rootfile full-path="OPS/book.opf"/></rootfiles></container>`
	for name, content := range files {
		out, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return filename
}

// assertNativeTitles 同时校验目录数量、锚点有效性和标题顺序，避免仅检查章名而漏掉行号错位。
func assertNativeTitles(t *testing.T, document Document, titles ...string) {
	t.Helper()
	if len(document.Native) != len(titles) || !chapterdetect.ValidateAnchors(document.Lines, document.Native) {
		t.Fatalf("native = %+v; lines = %q", document.Native, document.Lines)
	}
	for i, title := range titles {
		if document.Native[i].Title != title {
			t.Fatalf("title %d = %q, want %q", i, document.Native[i].Title, title)
		}
	}
}

// TestEPUB2NestedNCXAndMultipleAnchorsInOneFile 验证旧版嵌套目录、同文件多片段及同位置条目合并。
// 另检查正文锚点保留首段、行内文字连续，以及头部和脚本不进入正文。
func TestEPUB2NestedNCXAndMultipleAnchorsInOneFile(t *testing.T) {
	filename := writeEPUB(t, map[string]string{
		"OPS/book.opf":        `<package><manifest><item id="ncx" href="nav/toc.ncx" media-type="application/x-dtbncx+xml"/><item id="body" href="Text/body.xhtml"/></manifest><spine toc="ncx"><itemref idref="body"/></spine></package>`,
		"OPS/nav/toc.ncx":     `<ncx><navMap><navPoint><navLabel><text>第一卷</text></navLabel><content src="../Text/body.xhtml#c1"/><navPoint><navLabel><text>蓝色的门</text></navLabel><content src="../Text/body.xhtml#c1"/></navPoint></navPoint><navPoint><navLabel><text>灰色的路</text></navLabel><content src="../Text/body.xhtml#c2"/></navPoint></navMap></ncx>`,
		"OPS/Text/body.xhtml": `<html><head><title>不进入正文</title></head><body><h2 id="c1">蓝色的门</h2><p>第九章这个句子是正文，不能重新分章。</p><p id="c2">这是第二章的首段。</p><p>他<a href="#note">点了点头</a>，继续往前。</p><script>不进入正文</script></body></html>`,
	})
	document, err := ReadEPUB(filename)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "蓝色的门", "灰色的路")
	if !document.Native[1].KeepLine || document.Lines[document.Native[1].Line] != "这是第二章的首段。" ||
		!strings.Contains(document.Text(), "他点了点头，继续往前。") || strings.Contains(document.Text(), "不进入正文") {
		t.Fatalf("lost visible body text or retained executable metadata: %+v", document)
	}
}

// TestEPUB3NavigationWithEscapedFragmentsAndSpineOrder 验证新版目录的中文转义片段与正文阅读顺序。
// 样本故意让目录顺序与阅读顺序相反，防止误用目录排列或压缩包顺序输出正文。
func TestEPUB3NavigationWithEscapedFragmentsAndSpineOrder(t *testing.T) {
	filename := writeEPUB(t, map[string]string{
		"OPS/book.opf":          `<package><manifest><item id="nav" href="nav.xhtml" properties="nav"/><item id="first" href="Text/first.xhtml"/><item id="second" href="Text/second.xhtml"/></manifest><spine><itemref idref="second"/><itemref idref="first"/></spine></package>`,
		"OPS/nav.xhtml":         `<html xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/first.xhtml#%E8%93%9D">蓝色的门</a></li><li><a href="Text/second.xhtml#gray">灰色的路</a></li></ol></nav><nav epub:type="landmarks"><a href="Text/first.xhtml#ignored">不是章节</a></nav></body></html>`,
		"OPS/Text/first.xhtml":  `<html><body><h2 id="蓝">蓝色的门</h2><p>蓝色正文。</p></body></html>`,
		"OPS/Text/second.xhtml": `<html><body><h2 id="gray">灰色的路</h2><p>灰色正文。</p></body></html>`,
	})
	document, err := ReadEPUB(filename)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "灰色的路", "蓝色的门")
	if strings.Index(document.Text(), "灰色正文") > strings.Index(document.Text(), "蓝色正文") {
		t.Fatal("used ZIP or TOC order instead of the reading spine")
	}
}

// TestEPUBNavigationDocumentCanAlsoContainRealBody 验证导航文件同时包含正文时只跳过目录区域。
func TestEPUBNavigationDocumentCanAlsoContainRealBody(t *testing.T) {
	filename := writeEPUB(t, map[string]string{
		"OPS/book.opf":   `<package><manifest><item id="body" href="body.xhtml" properties="nav"/></manifest><spine><itemref idref="body"/></spine></package>`,
		"OPS/body.xhtml": `<html xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><a href="#c">真实标题</a></nav><h2 id="c">真实标题</h2><p>不能因为文件带 nav 属性就丢掉正文。</p></body></html>`,
	})
	document, err := ReadEPUB(filename)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "真实标题")
	if !strings.Contains(document.Text(), "不能因为文件带 nav 属性就丢掉正文。") {
		t.Fatal("dropped readable body from a navigation document")
	}
}

// TestMOBINamedNavigationAndBodyHyperlinks 验证命名目录可以生成边界，而普通脚注链接仍保留为正文。
func TestMOBINamedNavigationAndBodyHyperlinks(t *testing.T) {
	raw := `<html><head><title>书名</title></head><body><nav role="doc-toc"><a href="#blue">蓝色的门</a><a href="#gray">灰色的路</a></nav><h2 id="blue">蓝色的门</h2><p>第七章这个句子还是正文。</p><p id="gray">灰色正文。</p><a href="#note">脚注链接</a><p id="note">脚注正文。</p></body></html>`
	document, err := ReadMOBIHTML(strings.NewReader(raw), false)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "蓝色的门", "灰色的路")
	if !document.Native[1].KeepLine || !strings.Contains(document.Text(), "脚注链接") {
		t.Fatal("discarded a paragraph or used an ordinary hyperlink as a chapter")
	}
	plain, err := ReadMOBIHTML(strings.NewReader(`<body><a href="#note">脚注链接</a><p id="note">脚注正文。</p></body>`), false)
	if err != nil || len(plain.Native) != 0 {
		t.Fatalf("body hyperlinks became chapters: %+v %v", plain, err)
	}
}

// TestMOBIGuideFilePositions 验证导引指定的字节位置目录可定位章节，编码不可信时必须拒绝该目录。
func TestMOBIGuideFilePositions(t *testing.T) {
	// 使用定宽占位数值，填入真实偏移后文档长度不变，确保测试坐标准确。
	template := `<html><head><guide><reference type="toc" filepos="%010d"/></guide></head><body><div class="toc"><h1>目录</h1><a filepos="%010d">蓝色的门</a><a filepos="%010d">灰色的路</a></div><h2 id="blue">蓝色的门</h2><p>正文甲。</p><h2 id="gray">灰色的路</h2><p>正文乙。</p></body></html>`
	raw := fmt.Sprintf(template, 0, 0, 0)
	raw = fmt.Sprintf(template, strings.Index(raw, `<div class="toc">`), strings.Index(raw, `<h2 id="blue">`), strings.Index(raw, `<h2 id="gray">`))
	document, err := ReadMOBIHTML(strings.NewReader(raw), true)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "蓝色的门", "灰色的路")
	if !strings.Contains(document.Text(), "正文甲。") || !strings.Contains(document.Text(), "正文乙。") {
		t.Fatal("file position markers removed body text")
	}
	unsafeOffsets, err := ReadMOBIHTML(strings.NewReader(raw), false)
	if err != nil || len(unsafeOffsets.Native) > 0 {
		t.Fatal("treated offsets in a re-encoded document as verified positions")
	}
}

// TestBrokenNativeDirectoryFallsBackWithoutLosingText 验证外部链接和缺失片段不会损坏可供回退检测的正文。
func TestBrokenNativeDirectoryFallsBackWithoutLosingText(t *testing.T) {
	filename := writeEPUB(t, map[string]string{
		"OPS/book.opf":   `<package><manifest><item id="nav" href="nav.xhtml" properties="nav"/><item id="body" href="body.xhtml"/></manifest><spine><itemref idref="body"/></spine></package>`,
		"OPS/nav.xhtml":  `<html><body><nav role="doc-toc"><a href="https://example.com/book">外部链接</a><a href="body.xhtml#missing">坏链接</a></nav></body></html>`,
		"OPS/body.xhtml": `<html><body><h2>第一章 开始</h2><p>仍然保留全部正文。</p></body></html>`,
	})
	document, err := ReadEPUB(filename)
	if err != nil || len(document.Native) != 0 || !strings.Contains(document.Text(), "仍然保留全部正文。") {
		t.Fatalf("broken navigation damaged the text: %+v %v", document, err)
	}
}

// TestMalformedMOBIHTMLUsesStreamingFallback 以大量未闭合容器触发流式回退，检查目录和图片后正文均保留。
func TestMalformedMOBIHTMLUsesStreamingFallback(t *testing.T) {
	raw := `<html><body><nav role="doc-toc"><a href="#blue">蓝色的门</a><a href="#gray">灰色的路</a></nav>` +
		strings.Repeat("<div>", 1000) + `<h2 id="blue">蓝色的门</h2><p>第一段正文。</p><img src="missing.jpg"><h2 id="gray">灰色的路</h2><p>最后一段正文。</p>`
	document, err := ReadMOBIHTML(strings.NewReader(raw), false)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "蓝色的门", "灰色的路")
	if !strings.Contains(document.Text(), "第一段正文。") || !strings.Contains(document.Text(), "最后一段正文。") {
		t.Fatal("stream fallback discarded body after malformed containers or a void image")
	}
}

// TestPlainMOBITextKeepsPhysicalLines 验证纯文本电子书保留空行及全角缩进，供后续上下文检测使用。
func TestPlainMOBITextKeepsPhysicalLines(t *testing.T) {
	document, err := ReadMOBIHTML(strings.NewReader("第一章\r\n\r\n　　正文第一段。\r\n第二章\r\n正文第二段。\r\n"), false)
	if err != nil || len(document.Lines) != 5 || document.Lines[1] != "" || document.Lines[2] != "　　正文第一段。" {
		t.Fatalf("plain-text MOBI lost its paragraph context: %+v %v", document, err)
	}
}

// TestMalformedEPUBBodyStillUsesItsFileLevelDirectory 验证异常正文回退到流式解析后仍可使用文件入口目录。
func TestMalformedEPUBBodyStillUsesItsFileLevelDirectory(t *testing.T) {
	filename := writeEPUB(t, map[string]string{
		"OPS/book.opf":   `<package><manifest><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="body" href="body.xhtml"/></manifest><spine toc="ncx"><itemref idref="body"/></spine></package>`,
		"OPS/toc.ncx":    `<ncx><navMap><navPoint><navLabel><text>真实标题</text></navLabel><content src="body.xhtml"/></navPoint></navMap></ncx>`,
		"OPS/body.xhtml": `<html><head><title>书名</title></head><body>` + strings.Repeat("<div>", 1000) + `<h2>真实标题</h2><p>不能丢掉的正文。</p>`,
	})
	document, err := ReadEPUB(filename)
	if err != nil {
		t.Fatal(err)
	}
	assertNativeTitles(t, document, "真实标题")
	if !strings.Contains(document.Text(), "不能丢掉的正文。") {
		t.Fatal("stream fallback lost EPUB body text")
	}
}

// TestEmptyCoverAnchorCannotDriftIntoTheFollowingChapter 验证无可见文字的封面锚点不会漂移到下一正文文件。
func TestEmptyCoverAnchorCannotDriftIntoTheFollowingChapter(t *testing.T) {
	filename := writeEPUB(t, map[string]string{
		"OPS/book.opf":    `<package><manifest><item id="nav" href="nav.xhtml" properties="nav"/><item id="cover" href="cover.xhtml"/><item id="body" href="body.xhtml"/></manifest><spine><itemref idref="cover"/><itemref idref="body"/></spine></package>`,
		"OPS/nav.xhtml":   `<html><body><nav role="doc-toc"><a href="cover.xhtml">封面</a></nav></body></html>`,
		"OPS/cover.xhtml": `<html><body><img src="cover.jpg"/></body></html>`,
		"OPS/body.xhtml":  `<html><body><h2>第一章 开始</h2><p>真实的正文。</p></body></html>`,
	})
	document, err := ReadEPUB(filename)
	if err != nil || len(document.Native) != 0 || !strings.Contains(document.Text(), "真实的正文。") {
		t.Fatalf("empty cover anchor hijacked the following chapter: %+v %v", document, err)
	}
}
