package book

import (
	"archive/zip"
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"wd-reader/internal/book/mobi"
	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/chapterdetect/source"
)

// TestSmartSplitPreservesRejectedParagraphsAndExplicitInlineBodies 验证智能过滤完整保留正文，并兼容同行边界。
// 另外验证自定义正则和按段落模式不会受到智能过滤影响。
func TestSmartSplitPreservesRejectedParagraphsAndExplicitInlineBodies(t *testing.T) {
	paragraph := "　　第三章我们已经讨论过这个问题，" + strings.Repeat("他推开房门，发现外面下着雨。", 12) + "  "
	text := "第一章 初见\n" + paragraph + "\n第二章 夜雨\n完。"
	chapters, err := parseReadableChapters(bufio.NewScanner(strings.NewReader(text)), "1", "")
	if err != nil || len(chapters) != 2 || chapters[0].Name != "第一章 初见" || !reflect.DeepEqual(chapters[0].Body, []string{paragraph}) {
		t.Fatalf("paragraph was split, trimmed or lost: %+v %v", chapters, err)
	}
	body := strings.Repeat("他推开房门，发现外面下着雨。", 8)
	inline, err := parseReadableChapters(bufio.NewScanner(strings.NewReader("第一章 初见\t"+body)), "1", "")
	if err != nil || len(inline) != 1 || inline[0].Name != "第一章 初见" || strings.Join(inline[0].Body, "\n") != body {
		t.Fatalf("explicit inline boundary lost content: %+v %v", inline, err)
	}
	// 自定义正则由用户明确指定，即使命中正文式长行，也应按用户规则生成标题。
	custom, err := parseReadableChapters(bufio.NewScanner(strings.NewReader(text)), "3", "^第三章")
	if err != nil || len(custom) != 2 || custom[1].Name != strings.TrimSpace(paragraph) {
		t.Fatalf("smart filtering leaked into custom regex mode: %+v %v", custom, err)
	}
	paragraphs, err := parseReadableChapters(bufio.NewScanner(strings.NewReader(text)), "2", "2")
	if err != nil || len(paragraphs) != 2 || paragraphs[0].Body[0] != "第一章 初见" {
		t.Fatalf("smart filtering leaked into paragraph mode: %+v %v", paragraphs, err)
	}
}

// TestCenturyAZW3NativeChapters 验证真实无标题正文的二十章定位、首段保留及旧转换文本恢复。
// 新导入和旧书恢复均在临时书库中进行，不覆盖实际书籍与阅读进度。
func TestCenturyAZW3NativeChapters(t *testing.T) {
	original, err := filepath.Abs(filepath.Join("..", "..", "test", "百年孤独.azw3"))
	if _, err := os.Stat(original); os.IsNotExist(err) {
		t.Skip("未提供可选的真实书籍样本")
	}
	if err != nil {
		t.Fatal(err)
	}
	resourceLibrary(t)
	filename, err := ImportLocal(original)
	if err != nil {
		t.Fatal(err)
	}
	document, err := readNativeSource(original)
	if err != nil || len(document.Native) != 20 {
		t.Fatalf("原生目录=%d，错误=%v", len(document.Native), err)
	}
	assertChapters := func(name string) {
		t.Helper()
		chapters, err := readBookChapters(name, "1", "")
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, chapter := range chapters {
			if chapter.Name == "正文" {
				continue
			}
			if found >= len(document.Native) || chapter.Name != document.Native[found].Title || len(chapter.Body) < 5 {
				t.Fatalf("章节定位错误：%q，正文段数=%d", chapter.Name, len(chapter.Body))
			}
			first := chapterTextKey(document.Lines[document.Native[found].Line])
			if chapterTextKey(chapter.Body[0]) != first {
				t.Fatalf("章节%q首段丢失或错位", chapter.Name)
			}
			for _, line := range chapter.Body {
				if chapterTextKey(line) == "TableofContents" || chapterTextKey(line) == "第1章" {
					t.Fatalf("章节%q混入了书末目录", chapter.Name)
				}
			}
			found++
		}
		if found != 20 {
			t.Fatalf("可阅读章节=%d，期望20", found)
		}
		t.Logf("%s：20个章节均有正文，首段与原生目录位置一致", name)
	}
	assertChapters(filename)
	// 模拟修复前导入的拼接正文，验证按首段对齐可避开书末二十个同名目录标题。
	reader, err := mobi.NewReader(original)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := reader.ExtractHtml()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := source.ReadMOBIHTML(strings.NewReader(raw), true)
	if err != nil {
		t.Fatal(err)
	}
	// 排除旧拼接方式泄漏的头部书名及目录展示行后，整本可见文字必须与重建结果一致。
	// 此断言覆盖全部章节，而不是只验证二十个起点，防止分片重建遗漏中间正文。
	nativeTitles := map[string]bool{}
	for _, anchor := range document.Native {
		nativeTitles[chapterTextKey(anchor.Title)] = true
	}
	var legacyBody strings.Builder
	for _, line := range legacy.Lines {
		key := chapterTextKey(line)
		if key == "百年孤独（范晔译本）" || key == "TableofContents" || nativeTitles[key] {
			continue
		}
		legacyBody.WriteString(key)
	}
	if legacyBody.String() != chapterTextKey(document.Text()) {
		t.Fatal("重建结果与旧正文中的可见文字不一致")
	}
	putResource(t, filepath.Join(ParsePath(), "legacy.txt"), legacy.Text())
	if err := RecordImport("legacy.txt", filepath.Base(original)); err != nil {
		t.Fatal(err)
	}
	assertChapters("legacy.txt")
	if resourceContent(t, filepath.Join(ParsePath(), "legacy.txt")) != legacy.Text() {
		t.Fatal("恢复目录时改写了旧正文")
	}
}

// makeNativeEPUB 创建用于书库集成测试的电子书，含无编号原生标题及带章节前缀的真实正文。
// 此设计确保只有正确读取原生目录，才能同时得到预期章名和完整正文。
func makeNativeEPUB(t *testing.T) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "native.epub")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	files := map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OPS/book.opf"/></rootfiles></container>`,
		"OPS/book.opf":           `<package xmlns:dc="http://purl.org/dc/elements/1.1/"><metadata><dc:title>原生目录测试</dc:title></metadata><manifest><item id="nav" href="nav.xhtml" properties="nav"/><item id="body" href="body.xhtml"/></manifest><spine><itemref idref="body"/></spine></package>`,
		"OPS/nav.xhtml":          `<html xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><a href="body.xhtml#first">蓝色的门</a><a href="body.xhtml#second">灰色的路</a></nav></body></html>`,
		"OPS/body.xhtml":         `<html><body><h2 id="first">蓝色的门</h2><p>第一章这个整段只是正文，不能误识别。</p><h2 id="second">灰色的路</h2><p>第二段正文。</p></body></html>`,
	}
	for name, value := range files {
		out, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte(value)); err != nil {
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

// TestNativeDirectorySurvivesImportRenameAndMissingOriginal 验证导入后索引独立于文件名及原文件存在状态。
// 还覆盖自定义规则优先以及正文变化后旧索引失效的行为。
func TestNativeDirectorySurvivesImportRenameAndMissingOriginal(t *testing.T) {
	resourceLibrary(t)
	filename, err := ImportLocal(makeNativeEPUB(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := GetChapterListByFileNameExtract(filename, "1", ""); got != "蓝色的门\n灰色的路" {
		t.Fatalf("did not use native navigation: %q", got)
	}
	if got := GetChapterContentByChpaterNameExtract(filename, "蓝色的门", "1", ""); got != "第一章这个整段只是正文，不能误识别。" {
		t.Fatalf("native chapter lost body text: %q", got)
	}
	indexes, err := os.ReadDir(filepath.Join(metadataDir(), "chapter-index"))
	if err != nil || len(indexes) != 1 {
		t.Fatalf("native directory was not persisted: %v %v", indexes, err)
	}
	// 删除原文件并改名转换正文，确认后续目录读取确实使用指纹索引，而非再次解析原书。
	if err := os.Remove(filepath.Join(PurePath(), "native.epub")); err != nil {
		t.Fatal(err)
	}
	renamed := "renamed.txt"
	if err := os.Rename(filepath.Join(ParsePath(), filename), filepath.Join(ParsePath(), renamed)); err != nil {
		t.Fatal(err)
	}
	if got := GetChapterListByFileNameExtract(renamed, "1", ""); got != "蓝色的门\n灰色的路" {
		t.Fatalf("filename-bound index was lost after rename: %q", got)
	}
	if got := GetChapterListByFileNameExtract(renamed, "3", "^第一章"); !strings.Contains(got, "第一章这个整段只是正文") {
		t.Fatalf("native index overrode a custom regular expression: %q", got)
	}
	// 替换正文后不再匹配旧指纹，必须回退到当前文本中的实际章节。
	putResource(t, filepath.Join(ParsePath(), renamed), "第一章 新正文\n新的内容。\n第二章 继续\n后续。")
	if got := GetChapterListByFileNameExtract(renamed, "1", ""); got != "第一章 新正文\n第二章 继续" {
		t.Fatalf("stale index applied to modified text: %q", got)
	}
}

// TestOldConvertedBookRecoversNativeDirectoryWithoutRewritingText 验证旧版转换文本可按原生标题恢复目录。
// 旧文本故意保留内嵌目录且不采用新转换器分行，读取后必须保持原文件内容完全不变。
func TestOldConvertedBookRecoversNativeDirectoryWithoutRewritingText(t *testing.T) {
	resourceLibrary(t)
	original := makeNativeEPUB(t)
	if err := copyExclusive(original, filepath.Join(PurePath(), "native.epub")); err != nil {
		t.Fatal(err)
	}
	legacy := "目录\n蓝色的门\n灰色的路\n蓝色的门\n第一章这个整段只是正文，不能误识别。\n灰色的路\n第二段正文。\n"
	putResource(t, filepath.Join(ParsePath(), "old.txt"), legacy)
	if err := RecordImport("old.txt", "native.epub"); err != nil {
		t.Fatal(err)
	}
	if got := GetChapterListByFileNameExtract("old.txt", "1", ""); got != "蓝色的门\n灰色的路" {
		t.Fatalf("old source navigation was not recovered: %q", got)
	}
	if got := resourceContent(t, filepath.Join(ParsePath(), "old.txt")); got != legacy {
		t.Fatal("reading a legacy book rewrote its text")
	}
}

// TestNativeDirectoryAtParagraphKeepsTheFirstParagraph 验证目录直接指向段落时，首段不会被当作标题消耗。
func TestNativeDirectoryAtParagraphKeepsTheFirstParagraph(t *testing.T) {
	lines := []string{"序言内容。", "这段直接开始章节，没有单独的标题。", "后续。"}
	chapters, err := parseChapterLines(lines, "1", "", []chapterdetect.Anchor{{Line: 1, Title: "目录中的章名", KeepLine: true}})
	if err != nil || len(chapters) != 2 || chapters[1].Body[0] != lines[1] {
		t.Fatalf("native boundary consumed its first paragraph: %+v %v", chapters, err)
	}
}

// TestNativeIndexTravelsWithResourceArchive 验证原生索引随资源包导入后，即使移除原书仍可正常分章。
func TestNativeIndexTravelsWithResourceArchive(t *testing.T) {
	resourceLibrary(t)
	filename, err := ImportLocal(makeNativeEPUB(t))
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := ExportResources(t.Context(), archive, FrontendCache{}, nil); err != nil {
		t.Fatal(err)
	}
	// 切换到全新的临时书库后导入，排除沿用原书库磁盘索引的可能。
	resourceLibrary(t)
	plan, err := PrepareResourceImport(t.Context(), archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if err := plan.Commit(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(PurePath(), "native.epub")); err != nil {
		t.Fatal(err)
	}
	if got := GetChapterListByFileNameExtract(filename, "1", ""); got != "蓝色的门\n灰色的路" {
		t.Fatalf("resource import lost the native index: %q", got)
	}
}

// TestNativeSourceExistingBookFixtures 使用可选的真实书籍样本验证解码与智能分章的完整链路。
// 只在内存中解析，不导入或改写用户书库；本地没有样本时跳过，保证测试环境可移植。
func TestNativeSourceExistingBookFixtures(t *testing.T) {
	for _, name := range []string{"我无限回档,洞悉所有底牌.epub", "诡舍.azw3", "惊悚我只卖凶宅.mobi"} {
		t.Run(name, func(t *testing.T) {
			filename := filepath.Join("..", "..", "test", name)
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				t.Skip("optional original book fixture is not present")
			}
			document, err := readNativeSource(filename)
			if err != nil || len(document.Lines) == 0 {
				t.Fatalf("original source conversion: %v", err)
			}
			chapters, err := parseChapterLines(document.Lines, "1", "", document.Native)
			if err != nil || len(chapters) == 0 {
				t.Fatalf("original source produced no readable chapters: %v", err)
			}
			t.Logf("decoded lines=%d, native anchors=%d, readable chapters=%d", len(document.Lines), len(document.Native), len(chapters))
		})
	}
}
