package chapterdetect

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// IndexVersion 为原生目录索引格式版本；结构不兼容时应递增，旧版本索引会被忽略并重新恢复。
const IndexVersion = 1

// NativeIndex 将原生目录与特定的解码正文绑定，支持改名及资源包迁移后的复用。
// 持久化及文件读写由书库适配层负责，本包仅定义结构和校验规则。
type NativeIndex struct {
	Version     int      `json:"version"`     // 索引格式版本，读取时必须与当前版本一致。
	Fingerprint string   `json:"fingerprint"` // 解码后物理行的摘要，不包含文件名或路径。
	Anchors     []Anchor `json:"anchors"`     // 按物理行号严格递增的原生目录锚点。
}

// Fingerprint 根据解码后的物理行生成文本指纹，不受书籍改名及首行字节顺序标记影响。
// 调用方按行读取时已移除换行符，因此标准换行格式差异不会影响指纹。
// 空行、缩进及其他正文变化仍会改变指纹，使旧行号索引失效。
func Fingerprint(lines []string) string {
	hash := sha256.New()
	var length [8]byte
	for i, line := range lines {
		if i == 0 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		// 每行先写入字节长度再写内容，避免不同分行方式拼接出相同的摘要输入。
		binary.LittleEndian.PutUint64(length[:], uint64(len(line)))
		hash.Write(length[:])
		hash.Write([]byte(line))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// ValidateAnchors 验证非空目录的行号范围、严格递增顺序及标题有效性。
// 普通标题锚点还必须与目标行归一化后相同；KeepLine 锚点指向正文，不要求二者相等。
// 任意锚点不合格即拒绝整组，避免部分错位的目录混入结果。
func ValidateAnchors(lines []string, anchors []Anchor) bool {
	if len(anchors) == 0 {
		return false
	}
	last := -1
	for _, anchor := range anchors {
		if anchor.Line <= last || anchor.Line < 0 || anchor.Line >= len(lines) || TextKey(anchor.Title) == "" ||
			(!anchor.KeepLine && TextKey(lines[anchor.Line]) != TextKey(anchor.Title)) {
			return false
		}
		last = anchor.Line
	}
	return true
}

// AlignNativeTitles 为旧版转换正文提供保守的原生目录恢复，不会改写旧文本。
// 按目录顺序逐个精确匹配归一化标题，优先选择后接正文的位置，以绕过连续的内嵌目录标题。
// 全部标题均可按顺序定位时才返回新锚点，任一标题缺失则整体放弃，由调用方回退到文本检测。
func AlignNativeTitles(lines []string, anchors []Anchor) []Anchor {
	known := map[string]bool{}
	positions := map[string][]int{}
	bodyPositions := map[string][]int{}
	for _, anchor := range anchors {
		known[TextKey(anchor.Title)] = true
	}
	// 反向缓存每行之后的首个非空行，后续判断标题后是否有正文时无需重复跨越空行。
	nextVisible := make([]int, len(lines))
	lastVisible := -1
	for i := len(lines) - 1; i >= 0; i-- {
		nextVisible[i] = lastVisible
		if TextKey(lines[i]) != "" {
			lastVisible = i
		}
	}
	// 将所有匹配位置与后接正文的位置分别建索引，区分目录中的重复标题和正文标题。
	for i, line := range lines {
		key := TextKey(line)
		if known[key] {
			positions[key] = append(positions[key], i)
			if nextVisible[i] >= 0 && !known[TextKey(lines[nextVisible[i]])] {
				bodyPositions[key] = append(bodyPositions[key], i)
			}
		}
	}
	var result []Anchor
	last := -1
	for _, anchor := range anchors {
		chosen := -1
		key := TextKey(anchor.Title)
		// 候选行号天然有序，二分查找上一锚点之后的位置，避免同名标题导致反复线性扫描。
		for _, choices := range [][]int{bodyPositions[key], positions[key]} {
			if index := sort.SearchInts(choices, last+1); index < len(choices) {
				chosen = choices[index]
				break
			}
		}
		if chosen < 0 {
			return nil
		}
		result = append(result, Anchor{Line: chosen, Title: anchor.Title})
		last = chosen
	}
	return result
}
