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
	"wd-reader/internal/chapterdetect"
)

// 章节读取整体流程：扫描转换正文并保留物理行，按当前模式加载原生目录或编译自定义规则，
// 将智能检测输出的边界映射到行号，顺序组装标题与正文，最后统一处理空章节、重复内容和同名标题。
// 目录查询与章节内容查询共用此流程，保证展示的章名总能定位到相同正文。

// readableChapter 是书库层最终可阅读的章节，不包含只有标题而没有正文的空章节。
type readableChapter struct {
	Name string   // 展示和查询使用的唯一章名，同名不同内容会追加区分后缀。
	Body []string // 正文行；智能模式保留原行的缩进及尾随空白，不包含物理空行。
}

// chapterTextKey 复用独立模块的比较规则，使标题匹配、正文去重和原生目录校验保持一致。
func chapterTextKey(text string) string {
	return chapterdetect.TextKey(text)
}

// scanChapterLines 将扫描器内容读为物理行，保留空行及缩进，仅移除首行字节顺序标记。
// 扫描失败时整体返回错误，不用截断正文继续分章。
func scanChapterLines(scanner *bufio.Scanner) ([]string, error) {
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取书籍失败: %w", err)
	}
	if len(lines) > 0 {
		lines[0] = strings.TrimPrefix(lines[0], "\ufeff")
	}
	return lines, nil
}

// parseReadableChapters 为扫描器调用方提供统一分章入口，不负责原文件或目录索引读取。
// 目录和内容使用同一组可阅读章节，避免内嵌目录中的空标题遮蔽真正的正文章节。
func parseReadableChapters(scanner *bufio.Scanner, splitType, splitValue string) ([]readableChapter, error) {
	lines, err := scanChapterLines(scanner)
	if err != nil {
		return nil, err
	}
	return parseChapterLines(lines, splitType, splitValue, nil)
}

// smartChapterSplit 判断是否启用默认智能检测。
// 设置了按段落数或非空自定义正则时交由对应模式处理，不应用智能过滤；配置校验由解析入口负责。
func smartChapterSplit(splitType, splitValue string) bool {
	return !(splitType == "2" && splitValue != "0") && !(splitType == "3" && splitValue != "")
}

// parseChapterLines 将物理行组装为可阅读章节，是目录查询与正文查询共用的核心实现。
// splitType 为“2”且值非“0”时按段落数分章，为“3”且规则非空时使用自定义正则，其他情况智能检测。
// native 只应由智能模式传入；本函数不读写书籍文件，配置无效时返回可展示的错误。
func parseChapterLines(lines []string, splitType, splitValue string, native []chapterdetect.Anchor) ([]readableChapter, error) {
	var chapterPattern *regexp.Regexp
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
	// 先完成整本书的智能判定再组装正文，确保全书风格统计不会受到逐行输出时机影响。
	boundaries := map[int]chapterdetect.Boundary{}
	if smartChapterSplit(splitType, splitValue) {
		for _, boundary := range chapterdetect.Detect(lines, native).Boundaries {
			boundaries[boundary.Line] = boundary
		}
	}

	chapters := make([]readableChapter, 0)
	// 以归一化章名及正文摘要去重；同名但正文不同的章节必须保留，并生成唯一展示名。
	seen := make(map[string]map[[32]byte]bool)
	usedNames := make(map[string]bool)
	var name string
	var body []string
	nativeTitles := map[string]bool{}
	directoryLines := nativeDirectoryLines(lines, native)
	for _, anchor := range native {
		nativeTitles[chapterTextKey(anchor.Title)] = true
	}
	// 提交闭包统一处理空章节、旧正文中的目录前缀、重复章节及同名标题。
	// 空章节不提交，也不占用章名，允许后续真正的同名章节继续被查询到。
	flush := func() {
		if len(body) == 0 {
			return
		}
		baseName := name
		if baseName == "" {
			// 单独的目录标签不是可阅读前言，不应单独生成“正文”章节。
			label := strings.ToLower(chapterTextKey(strings.Join(body, "")))
			if label == "目录" || label == "contents" || label == "tableofcontents" {
				body = nil
				return
			}
			if len(nativeTitles) > 0 {
				// 旧转换正文可能在开头保留目录；只有标签及全部已验证目录标题组成的内容才跳过。
				// 含有任意其他文字时保留为前置正文，不把真实前言误当目录删除。
				isDirectory, hasLabel := true, false
				for _, line := range body {
					key := strings.ToLower(chapterTextKey(line))
					if key == "目录" || key == "contents" || key == "tableofcontents" {
						hasLabel = true
					} else if !nativeTitles[chapterTextKey(line)] {
						isDirectory = false
					}
				}
				if isDirectory && hasLabel {
					body = nil
					return
				}
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

	// 顺序消费原行；修剪后的 line 仅用于识别与兼容旧模式，智能模式正文继续使用 raw。
	for index, raw := range lines {
		// 旧转换文本可能保留书末目录；仅跳过与整份原生目录逐项一致的展示区域。
		if directoryLines[index] {
			continue
		}
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if chapterTextKey(line) == "" || strings.HasPrefix(line, "<img") || strings.HasPrefix(line, "<script") || strings.HasPrefix(line, "<style") {
			continue
		}
		if linesPerChapter > 0 {
			// 按段落分章完全沿用用户设置，标题形态不会触发额外分章。
			if len(body) == 0 {
				name = fmt.Sprintf("%d、", len(chapters))
			}
			body = append(body, line)
			if len(body) == linesPerChapter {
				flush()
			}
			continue
		}
		boundary, detected := boundaries[index]
		if chapterPattern != nil && chapterPattern.MatchString(line) {
			// 自定义规则是用户明确选择的边界，不再由智能评分否决。
			boundary, detected = chapterdetect.Boundary{Title: line}, true
		}
		if detected {
			// 正文出现前的重复同名标题合并处理，避免目录或重复排版造成空章节覆盖。
			if len(body) == 0 && !boundary.KeepLine && boundary.Body == "" && chapterTextKey(name) == chapterTextKey(boundary.Title) {
				continue
			}
			flush()
			name = boundary.Title
			if boundary.KeepLine {
				// 原生目录直接指向正文段落时，该行同时是章节起点和首段正文，必须完整保留。
				body = append(body, raw)
			} else if boundary.Body != "" {
				// 同行拆分只消耗已确认的标题部分，独立模块返回的正文继续归入新章。
				body = append(body, boundary.Body)
			}
			continue
		}
		if smartChapterSplit(splitType, splitValue) {
			// 所有未确认的候选与普通正文走同一路径，保留原始缩进及尾随空白，防止误判造成丢字。
			body = append(body, raw)
		} else {
			body = append(body, line)
		}
	}
	flush()
	return chapters, nil
}

// nativeDirectoryLines 找出旧正文中带明确目录标签、且连续列出全部原生标题的展示行。
// 必须完整匹配才标记；正文中偶尔出现章名或不完整的列表一律保留。
func nativeDirectoryLines(lines []string, native []chapterdetect.Anchor) map[int]bool {
	result := map[int]bool{}
	if len(native) < 2 || !chapterdetect.ValidateAnchors(lines, native) {
		return result
	}
	for i, line := range lines {
		label := strings.ToLower(chapterTextKey(line))
		if label != "目录" && label != "contents" && label != "tableofcontents" {
			continue
		}
		rows, next := []int{i}, 0
		for j := i + 1; j < len(lines) && next < len(native); j++ {
			key := chapterTextKey(lines[j])
			if key == "" {
				continue
			}
			if key != chapterTextKey(native[next].Title) || j == native[next].Line {
				break
			}
			rows = append(rows, j)
			next++
		}
		if next == len(native) {
			for _, row := range rows {
				result[row] = true
			}
		}
	}
	return result
}

// readBookChapters 读取书库中的转换正文，仅在智能模式下恢复原生目录，再交给统一解析入口。
// 保留物理行后才能正确验证目录指纹和行号，因此不在扫描阶段预先去空行或裁剪正文。
func readBookChapters(fileName, splitType, splitValue string) ([]readableChapter, error) {
	path := filepath.Join(ParsePath(), fileName)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := GetScanner(path, file)
	// 转换书籍可能包含超过扫描器默认上限的长段落，提高缓冲上限并保留明确的读取失败提示。
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lines, err := scanChapterLines(scanner)
	if err != nil {
		return nil, err
	}
	var native []chapterdetect.Anchor
	if smartChapterSplit(splitType, splitValue) {
		native = nativeBookChapters(fileName, lines)
	}
	return parseChapterLines(lines, splitType, splitValue, native)
}
