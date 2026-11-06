// Package chapterdetect 提供独立的章节边界检测，不依赖书籍文件读写、书库、桌面绑定或前端设置。
//
// 整体流程：调用方保留物理空行和缩进并传入正文；本包优先验证原生目录，
// 无可靠目录时依次执行候选提取、整行特征评分、全书风格统计和上下文复核，
// 最终只返回可信的章节边界。仅含卷级入口的目录会与文本检测结果合并。
//
// 本包不删除或改写输入行。候选未被接受时，调用方必须将原行保留为正文；
// 只有明确的同行分隔才返回拆出的正文，原生目录指向段落时通过 KeepLine 提醒调用方保留该行。
package chapterdetect

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultPattern 为旧调用方提供统一的章节前缀正则；命中前缀不等于最终确认章节。
// 智能检测额外识别序章等特殊标题，并在候选阶段后继续执行评分和上下文校验。
const DefaultPattern = `^(?:(?:第\s*)?[零〇一二三四五六七八九十百千万亿两壹贰叁肆伍陆柒捌玖拾佰仟\d]+\s*[卷巻章节回篇集部]|[零〇一二三四五六七八九十百千万亿\d]+[、：:]|(?i:NO)\s*\d+[、：:. ]|(?i:chapter|book|part)\s+(?i:[ivxlcdm]+|\d+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)\b)`

var (
	// 前缀规则用于区分中文、英文、普通枚举及特殊标题，编号统一交由 ordinal 解析。
	chinesePrefix  = regexp.MustCompile(`^(第\s*)?([零〇一二三四五六七八九十百千万亿两壹贰叁肆伍陆柒捌玖拾佰仟\d]+)\s*([卷巻章节回篇集部])`)
	englishPrefix  = regexp.MustCompile(`(?i)^(chapter|book|part)\s+([ivxlcdm]+|\d+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)\b`)
	listPrefix     = regexp.MustCompile(`^(?:[Nn][Oo]\s*)?([零〇一二三四五六七八九十百千万亿\d]+)[、：:. ]`)
	specialPrefix  = regexp.MustCompile(`^(序章|楔子|序言|前言|引子|终章|尾声|后记|番外)(?:$|[\s：:、—－-]|[零〇一二三四五六七八九十百千万亿\d]+)`)
	bodySeparator  = regexp.MustCompile(`\t+|[ 　]{2,}`)
	proseReference = regexp.MustCompile(`^(中|里|的内容|我们已经|我们曾经|我们讨论|提到|讲过|所述|所说|讲述了|介绍了)`)
	englishProse   = regexp.MustCompile(`^(discusses|describes|explains|contains|shows|was|is|has|had|of the)\b`)
)

// Anchor 表示原书目录在解码正文中的物理行定位，使用前必须通过 ValidateAnchors 校验。
// 标题可能来自目录文本，未必等于目标行；目录指向正文段落时必须设置 KeepLine。
type Anchor struct {
	Line     int    `json:"line"`               // 从零开始的物理行号，包含输入中的空行。
	Title    string `json:"title"`              // 展示给读者的原生目录标题。
	KeepLine bool   `json:"keepLine,omitempty"` // 为真时目标行属于正文，分章时不得丢弃。
}

// Boundary 是最终接受的章节边界，调用方据此结束上一章并开始新章。
type Boundary struct {
	Line     int    // 边界所在的原始物理行号，从零开始。
	Title    string // 本章标题，可能来自正文标题行或原生目录。
	Body     string // 仅在同一行具有明确标题与正文分隔时保存拆出的正文。
	KeepLine bool   // 为真时保留整个原行，而不是将其作为标题行消耗掉。
	Native   bool   // 标记该边界是否直接来自验证通过的原生目录。
}

// Candidate 保存候选边界及其证据，供检测、测试、诊断和后续人工纠错使用。
// Reasons 使用稳定的内部原因标识；未接受的候选只用于解释判断，不表示应删除原行。
type Candidate struct {
	Boundary             // 候选所在行、标题和可选的同行正文。
	Style       string   // 标题样式，如中文章、英文章、普通枚举或特殊标题。
	Number      int      // 统一后的章节编号；没有可用编号时为负数。
	Score       int      // 累积可信度分数，结合初始格式与后续证据得到。
	Accepted    bool     // 最终是否通过分数门槛及普通枚举的额外校验。
	Reasons     []string // 加分、减分和特殊拒绝原因，保留处理顺序。
	length      int      // 标题的 Unicode 字符数，不是 UTF-8 字节数。
	sentences   int      // 连续句末标点合并后的句末组数。
	commas      int      // 逗号、分号数量，用于识别正文式叙述。
	blankBefore bool     // 前一物理行是否为空；文件起始位置也视为具有前置留白。
	blankAfter  bool     // 后一物理行是否为空。
	indented    bool     // 原行是否包含前导空白，作为段落形态的辅助证据。
}

// Result 汇总检测结果；Boundaries 用于分章，Candidates 用于解释文本检测过程。
// 完整原生目录直接生效时不会运行候选评分，因此 Candidates 可以为空。
type Result struct {
	Boundaries []Boundary  // 按物理行顺序排列的有效章节边界。
	Candidates []Candidate // 文本扫描得到的全部候选，包括被拒绝的候选。
	UsedNative bool        // 是否使用了校验通过的原生目录，包括卷级目录的合并情况。
}

// TextKey 去除 Unicode 空白、零宽空格和字节顺序标记，生成标题及正文比较用的键。
// 此操作只用于比较，不应用于替换实际展示或保存的正文。
func TextKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\u200b' || r == '\ufeff' {
			return -1
		}
		return r
	}, text)
}

// sentenceFeatures 统计句末标点组及逗号、分号数量，用于估计一行是否呈现正文句群形态。
func sentenceFeatures(text string) (sentences, commas int) {
	previousEnd := false
	for _, r := range text {
		end := strings.ContainsRune("。！？.!?", r)
		if end && !previousEnd {
			sentences++
		}
		if strings.ContainsRune("，,；;", r) {
			commas++
		}
		// 连续的问号、叹号及其后的引号、括号仍属于同一组句末，避免重复计数。
		previousEnd = end || (previousEnd && strings.ContainsRune("”’\"'）)", r))
	}
	return
}

// prefix 提取标题前缀的样式、编号、结束字节偏移及是否属于强格式。
// 返回的 end 与正则匹配结果一致，供字符串切片使用；未知样式返回空字符串和负编号。
func prefix(text string) (style string, number, end int, strong bool) {
	if match := chinesePrefix.FindStringSubmatchIndex(text); match != nil {
		style = "zh:" + text[match[6]:match[7]]
		return style, ordinal(text[match[4]:match[5]]), match[1], match[2] >= 0
	}
	if match := englishPrefix.FindStringSubmatchIndex(text); match != nil {
		return "en:" + strings.ToLower(text[match[2]:match[3]]), ordinal(text[match[4]:match[5]]), match[1], true
	}
	if match := specialPrefix.FindStringIndex(text); match != nil {
		return "special", -1, match[1], true
	}
	if match := listPrefix.FindStringSubmatchIndex(text); match != nil {
		return "list", ordinal(text[match[2]:match[3]]), match[1], false
	}
	return "", -1, 0, false
}

// unambiguousBody 仅在标题后存在制表符或多个空格，且尾部明显为正文时拆分同行内容。
// prefixEnd 为前缀的结束字节偏移；边界含糊时将整行作为候选标题继续评分，不猜测截断位置。
func unambiguousBody(text string, prefixEnd int) (title, body string) {
	for _, separator := range bodySeparator.FindAllStringIndex(text, -1) {
		if separator[0] <= prefixEnd || TextKey(text[prefixEnd:separator[0]]) == "" {
			continue
		}
		// 分隔符前必须有实际标题文字，且短标题后的尾部满足句群或长段落特征。
		head, tail := strings.TrimSpace(text[:separator[0]]), strings.TrimSpace(text[separator[1]:])
		ends, commas := sentenceFeatures(tail)
		if utf8.RuneCountInString(head) <= 60 && ((ends >= 2 && utf8.RuneCountInString(tail) >= 25) ||
			(utf8.RuneCountInString(tail) > 80 && (ends > 0 || commas >= 2))) {
			return head, tail
		}
	}
	return text, ""
}

// addEvidence 同时累加分数并记录原因，保证检测结果可以追溯到具体判断依据。
func addEvidence(candidate *Candidate, value int, reason string) {
	candidate.Score += value
	candidate.Reasons = append(candidate.Reasons, reason)
}

// coarseNativeDirectory 判断原生目录是否仅包含正文入口或卷、部等粗粒度条目。
// 这类目录只能补充定位信息，不能代替正文中的章级边界；调用前应先校验锚点。
func coarseNativeDirectory(anchors []Anchor) bool {
	for _, anchor := range anchors {
		title := strings.ToLower(TextKey(anchor.Title))
		if title == "start" || title == "body" || title == "contents" || title == "正文" || title == "开始阅读" {
			continue
		}
		style, _, _, _ := prefix(strings.TrimSpace(anchor.Title))
		if style != "zh:卷" && style != "zh:巻" && style != "zh:部" && style != "en:book" && style != "en:part" {
			return false
		}
	}
	return true
}

// Detect 综合整行特征、全书风格和局部上下文判断章节，不修改传入的正文和目录。
// lines 必须保留原始物理行、空行及缩进；native 可以为空，所有锚点均通过验证后才参与判断。
// 完整原生目录直接作为结果；缺失或失效时使用文本判断；仅有卷级入口时合并两者。
func Detect(lines []string, native []Anchor) Result {
	result := Result{Boundaries: []Boundary{}, Candidates: []Candidate{}}
	// 第一阶段：验证目录是否与当前正文对应，避免失效行号误切正文。
	if ValidateAnchors(lines, native) {
		result.UsedNative = true
		if !coarseNativeDirectory(native) {
			for _, anchor := range native {
				result.Boundaries = append(result.Boundaries, Boundary{Line: anchor.Line, Title: anchor.Title, KeepLine: anchor.KeepLine, Native: true})
			}
			return result
		}
	}
	// 第二阶段：缓存空行位置，逐行提取候选，先依据标题自身特征建立初始分数。
	blank := make([]bool, len(lines))
	for i, line := range lines {
		blank[i] = TextKey(line) == ""
	}
	styles := map[string]int{}
	lengths := []int{}
	for i, raw := range lines {
		text := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		style, number, end, strong := prefix(text)
		if style == "" {
			continue
		}
		title, body := unambiguousBody(text, end)
		candidate := Candidate{Boundary: Boundary{Line: i, Title: title, Body: body}, Style: style, Number: number,
			length: utf8.RuneCountInString(title), blankBefore: i == 0 || blank[i-1],
			blankAfter: i+1 < len(lines) && blank[i+1], indented: len(raw) != len(strings.TrimLeftFunc(raw, unicode.IsSpace))}
		candidate.sentences, candidate.commas = sentenceFeatures(title)
		// 强格式标题初始为四分，普通枚举仅为一分，其他已识别前缀为三分；仍需额外证据支持。
		if strong {
			candidate.Score = 4
		} else if style == "list" {
			candidate.Score = 1
		} else {
			candidate.Score = 3
		}
		// 长度只影响可信度，不作为绝对否决条件，保留合法长标题被全书风格证实的机会。
		if candidate.length <= 60 {
			addEvidence(&candidate, 1, "short-title")
		}
		if candidate.length > 80 {
			addEvidence(&candidate, -2, "long-line")
		}
		if candidate.length > 120 {
			addEvidence(&candidate, -2, "paragraph-length")
		}
		// 较长行包含多个句末组时更像完整段落；短标题中的问句和感叹句不会单独因此被否决。
		if candidate.sentences >= 2 && candidate.length > 35 {
			addEvidence(&candidate, -5, "multiple-sentences")
		} else if candidate.sentences > 0 && candidate.length > 80 {
			addEvidence(&candidate, -2, "prose-sentence")
		}
		if candidate.commas >= 3 && candidate.length > 45 {
			addEvidence(&candidate, -2, "prose-punctuation")
		}
		// “第一章中提到……”等正文引用不应仅因章节前缀而变成标题。
		if end < len(title) && !unicode.IsSpace([]rune(title[end:])[0]) &&
			(candidate.sentences > 0 || candidate.commas > 0) && proseReference.MatchString(title[end:]) {
			addEvidence(&candidate, -5, "prose-reference")
		}
		if strings.HasPrefix(style, "en:") && end < len(title) && candidate.sentences > 0 && englishProse.MatchString(strings.TrimSpace(title[end:])) {
			addEvidence(&candidate, -5, "english-prose-reference")
		}
		// 上下留白分别提供辅助加分，不要求真实标题必须同时具有两侧空行。
		if candidate.blankBefore {
			addEvidence(&candidate, 1, "blank-before")
		}
		if candidate.blankAfter {
			addEvidence(&candidate, 1, "blank-after")
		}
		// 全书风格只从相对可靠的候选中统计，避免大量正文误匹配反过来强化错误样式。
		if candidate.sentences <= 1 &&
			((candidate.length <= 80 && candidate.Score >= 4) ||
				(strong && candidate.length <= 240 && candidate.sentences == 0 && candidate.commas < 3) ||
				(style == "list" && candidate.length <= 30 && candidate.sentences == 0)) {
			styles[style]++
			lengths = append(lengths, candidate.length)
		}
		result.Candidates = append(result.Candidates, candidate)
	}
	// 第三阶段：以可信候选的标题长度中位数和占比识别全书主流标题形态。
	sort.Ints(lengths)
	median := 0
	if len(lengths) > 0 {
		median = lengths[len(lengths)/2]
	}
	dominant := ""
	// 至少出现三次且占可信样本的六成才视为主流样式，避免小样本武断限制全书标题。
	for style, count := range styles {
		if count >= 3 && count*5 >= len(lengths)*3 {
			dominant = style
		}
	}
	// 一次性建立行号索引及同样式的后继编号，避免逐个候选反复扫描全部候选列表。
	byLine := map[int]bool{}
	styleByLine := map[int]string{}
	previous := map[string]int{}
	for _, candidate := range result.Candidates {
		byLine[candidate.Line] = true
		styleByLine[candidate.Line] = candidate.Style
	}
	nextNumber := make([]int, len(result.Candidates))
	next := map[string]int{}
	for i := len(result.Candidates) - 1; i >= 0; i-- {
		candidate := result.Candidates[i]
		nextNumber[i] = -1
		if value, ok := next[candidate.Style]; ok {
			nextNumber[i] = value
		}
		// 后继编号仅采纳初始证据达到四分的候选，降低可疑正文对编号连续性判断的干扰。
		if candidate.Score >= 4 {
			next[candidate.Style] = candidate.Number
		}
	}
	// 第四阶段：加入全书风格、局部留白、缩进及相邻编号证据，再作最终判定。
	for i := range result.Candidates {
		candidate := &result.Candidates[i]
		if styles[candidate.Style] >= 2 {
			addEvidence(candidate, 2, "consistent-style")
		}
		// 已有明确章节风格的书中，异类枚举和不同语言前缀降低可信度；序章、后记等保留独立样式。
		if dominant != "" && candidate.Style == "list" && dominant != "list" {
			addEvidence(candidate, -3, "foreign-numbering-style")
		}
		if dominant != "" && candidate.Style != "special" &&
			((strings.HasPrefix(dominant, "zh:") && strings.HasPrefix(candidate.Style, "en:")) ||
				(strings.HasPrefix(dominant, "en:") && strings.HasPrefix(candidate.Style, "zh:"))) {
			addEvidence(candidate, -2, "foreign-title-style")
		}
		// 长度远超全书典型标题，或无留白的长缩进段落，只作轻度降权，不设置绝对长度禁区。
		if median > 0 && candidate.length > 60 && candidate.length > median*3 {
			addEvidence(candidate, -1, "length-outlier")
		}
		if candidate.indented && !candidate.blankBefore && !candidate.blankAfter && candidate.length > 40 {
			addEvidence(candidate, -1, "paragraph-indentation")
		}
		// 编号连续只加分，不连续不扣分，允许缺章、番外以及分卷后的编号重置。
		if value, ok := previous[candidate.Style]; ok && candidate.Number >= 0 && candidate.Number == value+1 {
			addEvidence(candidate, 1, "previous-number")
		}
		if candidate.Number >= 0 && nextNumber[i] == candidate.Number+1 {
			addEvidence(candidate, 1, "next-number")
		}
		// 五分为基本接收门槛；普通枚举即使达到门槛，也必须再通过后续正文及非连续列表校验。
		candidate.Accepted = candidate.Score >= 5
		if candidate.Style == "list" {
			if candidate.blankBefore && candidate.blankAfter {
				addEvidence(candidate, 1, "standalone-enumeration")
				candidate.Accepted = candidate.Score >= 5
			}
			// 普通枚举必须同时具有独立证据和后续正文，否则连续列表项会被当作空章节丢掉。
			// 此处只检查首个后续非空行，不要求正文达到固定长度，避免误伤短章。
			bodyFollows := candidate.Body != ""
			for line := candidate.Line + 1; !bodyFollows && line < len(lines); line++ {
				if blank[line] {
					continue
				}
				bodyFollows = !byLine[line]
				break
			}
			candidate.Accepted = candidate.Accepted && bodyFollows &&
				(styles["list"] >= 2 || (candidate.blankBefore && candidate.blankAfter))
			// 同时检查前一个非空行，防止列表末项因后面恰有普通段落而被接受。
			for line := candidate.Line - 1; line >= 0; line-- {
				if blank[line] {
					continue
				}
				if styleByLine[line] == "list" {
					candidate.Accepted = false
					candidate.Reasons = append(candidate.Reasons, "consecutive-enumeration")
				}
				break
			}
		}
		if candidate.Accepted {
			if candidate.Number >= 0 {
				previous[candidate.Style] = candidate.Number
			}
			result.Boundaries = append(result.Boundaries, candidate.Boundary)
		}
	}
	if result.UsedNative {
		// 第五阶段：将正文入口或卷级目录补入章级检测结果，同一行优先保留文本识别的章标题。
		// 合并后按物理位置排序，保证调用方可以一次顺序扫描完成分章。
		occupied := map[int]bool{}
		for _, boundary := range result.Boundaries {
			occupied[boundary.Line] = true
		}
		for _, anchor := range native {
			if !occupied[anchor.Line] {
				result.Boundaries = append(result.Boundaries, Boundary{Line: anchor.Line, Title: anchor.Title, KeepLine: anchor.KeepLine, Native: true})
			}
		}
		sort.Slice(result.Boundaries, func(i, j int) bool { return result.Boundaries[i].Line < result.Boundaries[j].Line })
	}
	return result
}
