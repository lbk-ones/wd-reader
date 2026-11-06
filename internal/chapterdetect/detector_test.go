package chapterdetect

import (
	"reflect"
	"strings"
	"testing"
)

// titles 按检测结果顺序提取章名，简化边界和顺序的断言。
func titles(result Result) []string {
	values := []string{}
	for _, boundary := range result.Boundaries {
		values = append(values, boundary.Title)
	}
	return values
}

// TestLegacyBodyAnchorUsesParagraphInsteadOfTOC 验证正文锚点按首段对齐，拒绝用书末目录标题替代。
func TestLegacyBodyAnchorUsesParagraphInsteadOfTOC(t *testing.T) {
	reference := []string{"首章首段。", "后续正文。", "次章首段。"}
	anchors := []Anchor{{Line: 0, Title: "第1章", KeepLine: true}, {Line: 2, Title: "第2章", KeepLine: true}}
	legacy := []string{"旧版前言", "首章首段。", "后续正文。", "次章首段。", "目录", "第1章", "第2章"}
	got := AlignNativeLines(legacy, reference, anchors)
	if len(got) != 2 || got[0].Line != 1 || got[1].Line != 3 || !got[0].KeepLine {
		t.Fatalf("旧正文定位错误：%+v", got)
	}
	if got := AlignNativeLines([]string{"目录", "第1章", "第2章"}, reference, anchors); got != nil {
		t.Fatalf("用目录替代了正文：%+v", got)
	}
	if got := AlignNativeTitles(legacy, anchors); got != nil {
		t.Fatalf("不应按标题恢复正文锚点：%+v", got)
	}
}

// TestDetectWholeLinesAndBookContext 覆盖整行评分、长短标题、全书风格及编号上下文的组合行为。
// 同时包含真实章节和形似章节的正文，防止过滤误判时损伤合法标题。
func TestDetectWholeLinesAndBookContext(t *testing.T) {
	longParagraph := "第三章我们已经讨论过这个问题，" + strings.Repeat("他推开房门，发现外面下着雨。", 20)
	longTitle := strings.Repeat("风起云涌", 28)
	for _, tc := range []struct {
		name string   // 子用例名称，描述本次验证的正文或标题特征。
		text string   // 包含原始换行的完整样本，不提前移除空行。
		want []string // 预期接受的章名，顺序必须与正文一致。
	}{
		{"paragraph prefix", "第一章 初见\n正文。\n" + longParagraph + "\n第二章 夜雨\n正文。", []string{"第一章 初见", "第二章 夜雨"}},
		{"short prose reference", "第一章 初见\n正文。\n第三章我们已经讨论过这个问题。\n第二章 夜雨\n正文。", []string{"第一章 初见", "第二章 夜雨"}},
		{"English prose reference", "Chapter One\nBody.\nChapter Two discusses this problem.\nChapter Three\nBody.", []string{"Chapter One", "Chapter Three"}},
		{"punctuation in real titles", "第五章 “你是谁？”\n嗯。\n第六章 为什么？怎么办？\n完。", []string{"第五章 “你是谁？”", "第六章 为什么？怎么办？"}},
		{"Unicode character length", "第一章 " + strings.Repeat("雨", 45) + "\n正文。\n第二章 结束\n完。", []string{"第一章 " + strings.Repeat("雨", 45), "第二章 结束"}},
		{"long consistent titles", "第一章 " + longTitle + "\n\n正文。\n\n第二章 " + longTitle + "\n\n正文。", []string{"第一章 " + longTitle, "第二章 " + longTitle}},
		{"volume resets and missing numbers", "第一卷\n介绍。\n第一章\n正文。\n第三章\n正文。\n第二卷\n介绍。\n第一章\n正文。", []string{"第一卷", "第一章", "第三章", "第二卷", "第一章"}},
		{"special chapters", "序章\n开头。\n第一章 开始\n正文。\n番外一 夜雨\n额外的故事。\n后记\n作者的话。", []string{"序章", "第一章 开始", "番外一 夜雨", "后记"}},
		{"real numbered titles", "一、夜雨\n故事开始。\n二、重逢\n故事继续。", []string{"一、夜雨", "二、重逢"}},
		{"ordinary consecutive list", "一、检查门窗\n二、收拾物品\n三、离开房间\n他照做了。", []string{}},
		{"foreign list in chaptered book", "第一章\n正文。\n一、打开门\n接着，他走了出去。\n第二章\n正文。\n二、关上窗\n接着，他走了出去。\n第三章\n正文。", []string{"第一章", "第二章", "第三章"}},
		{"body without headings", "这是正文。\n这是另一段。", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Detect(strings.Split(tc.text, "\n"), nil)
			if !reflect.DeepEqual(titles(got), tc.want) {
				t.Fatalf("boundaries = %q, want %q; evidence = %+v", titles(got), tc.want, got.Candidates)
			}
		})
	}
}

// TestExplicitSameLineBodyBoundary 验证明确制表符可拆出同行正文，普通单空格不会触发猜测截断。
func TestExplicitSameLineBodyBoundary(t *testing.T) {
	body := strings.Repeat("他推开房门，发现外面下着雨。", 8)
	result := Detect([]string{"第一章 初见\t" + body}, nil)
	if len(result.Boundaries) != 1 || result.Boundaries[0].Title != "第一章 初见" || result.Boundaries[0].Body != body {
		t.Fatalf("explicit boundary = %+v", result)
	}
	ambiguous := "第一章 " + body
	if result := Detect([]string{ambiguous}, nil); len(result.Boundaries) != 0 {
		t.Fatalf("invented a boundary inside a paragraph: %+v", result)
	}
}

// TestNativePositionsOverrideHeuristicsAndPreserveBody 验证原生目录优先、首段保留及失效位置回退。
func TestNativePositionsOverrideHeuristicsAndPreserveBody(t *testing.T) {
	lines := []string{"没有编号的标题", "第一章这个句子属于正文。", "这是下一章的第一段。"}
	anchors := []Anchor{{Line: 0, Title: lines[0]}, {Line: 2, Title: "另一个标题", KeepLine: true}}
	result := Detect(lines, anchors)
	if !result.UsedNative || len(result.Boundaries) != 2 || !result.Boundaries[1].KeepLine {
		t.Fatalf("native directory was ignored: %+v", result)
	}
	if result := Detect(lines, []Anchor{{Line: 99, Title: "stale"}}); result.UsedNative {
		t.Fatal("trusted an out-of-range native index")
	}
	if result := Detect(lines, []Anchor{{Line: 0, Title: "wrong title"}}); result.UsedNative {
		t.Fatal("trusted a native title that no longer matches the text")
	}
}

// TestCoarseOriginalDirectoryDoesNotCollapseAllChapters 验证卷级或正文入口目录不会吞掉真实章级边界。
func TestCoarseOriginalDirectoryDoesNotCollapseAllChapters(t *testing.T) {
	lines := []string{"第一卷", "介绍。", "第一章 开始", "正文。", "第二章 继续", "正文。"}
	result := Detect(lines, []Anchor{{Line: 0, Title: "第一卷"}})
	if !result.UsedNative || !reflect.DeepEqual(titles(result), []string{"第一卷", "第一章 开始", "第二章 继续"}) {
		t.Fatalf("volume-only native directory swallowed chapter boundaries: %+v", result)
	}
	result = Detect(lines[2:], []Anchor{{Line: 0, Title: "Start", KeepLine: true}})
	if !reflect.DeepEqual(titles(result), []string{"第一章 开始", "第二章 继续"}) {
		t.Fatalf("generic Start entry swallowed chapter boundaries: %+v", result)
	}
}

// TestLegacyNativeAlignmentBypassesEmbeddedDirectory 验证旧正文对齐优先选择后接正文的标题，且拒绝缺失标题。
func TestLegacyNativeAlignmentBypassesEmbeddedDirectory(t *testing.T) {
	lines := []string{"目录", "蓝色的门", "灰色的路", "", "蓝色的门", "", "真正的正文。", "灰色的路", "另一段正文。"}
	anchors := AlignNativeTitles(lines, []Anchor{{Title: "蓝色的门"}, {Title: "灰色的路"}})
	if !reflect.DeepEqual(anchors, []Anchor{{Line: 4, Title: "蓝色的门"}, {Line: 7, Title: "灰色的路"}}) {
		t.Fatalf("aligned with the wrong title occurrences: %+v", anchors)
	}
	if got := AlignNativeTitles(lines, []Anchor{{Title: "不存在"}}); got != nil {
		t.Fatal("accepted an incomplete native alignment")
	}
}

// TestFingerprintAndOrdinals 验证指纹区分物理分行，并兼容字节顺序标记及多种章节编号写法。
func TestFingerprintAndOrdinals(t *testing.T) {
	if Fingerprint([]string{"\ufeff第一章", "正文"}) != Fingerprint([]string{"第一章", "正文"}) ||
		Fingerprint([]string{"a", "bc"}) == Fingerprint([]string{"ab", "c"}) {
		t.Fatal("fingerprints do not bind physical line positions")
	}
	for text, want := range map[string]int{"十二": 12, "壹佰零贰": 102, "两千三百": 2300, "一亿二万": 100020000, "二〇二四": 2024, "XIV": 14, "twenty": 20, "42": 42} {
		if got := ordinal(text); got != want {
			t.Fatalf("ordinal(%q) = %d, want %d", text, got, want)
		}
	}
}
