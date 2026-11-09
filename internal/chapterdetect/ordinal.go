package chapterdetect

import (
	"strconv"
	"strings"
)

// Ordinal parses a chapter number in the same formats used by the detector.
func Ordinal(text string) int {
	return ordinal(text)
}

// ordinal 将阿拉伯数字、英文数词、罗马数字及中文数字统一为整数，用于相邻编号加分。
// 中文同时支持常用字、大写数字和逐位写法；无法识别的非空输入返回负数，不据此拒绝标题。
func ordinal(text string) int {
	if value, err := strconv.Atoi(text); err == nil {
		return value
	}
	english := strings.Fields("zero one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty")
	for number, word := range english {
		if strings.EqualFold(text, word) {
			return number
		}
	}
	// 罗马数字遇到后一个字符更大时，减回前一个字符的两倍，以处理先减后加的记法。
	roman := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	value, last := 0, 0
	for _, r := range strings.ToUpper(text) {
		current, ok := roman[r]
		if !ok {
			value = -1
			break
		}
		value += current
		if current > last {
			value -= 2 * last
		}
		last = current
	}
	if value > 0 {
		return value
	}
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '壹': 1, '二': 2, '两': 2, '贰': 2, '三': 3, '叁': 3, '四': 4, '肆': 4, '五': 5, '伍': 5, '六': 6, '陆': 6, '七': 7, '柒': 7, '八': 8, '捌': 8, '九': 9, '玖': 9}
	units := map[rune]int{'十': 10, '拾': 10, '百': 100, '佰': 100, '千': 1000, '仟': 1000, '万': 10000, '亿': 100000000}
	// 中文数字分为总值、万或亿以内的分节和当前数字；无单位时按逐位数字累积。
	total, section, number := 0, 0, 0
	hasUnit := strings.ContainsAny(text, "十拾百佰千仟万亿")
	for _, r := range text {
		if digit, ok := digits[r]; ok {
			if hasUnit {
				number = digit
			} else {
				number = number*10 + digit
			}
		} else if unit, ok := units[r]; ok {
			if unit >= 10000 {
				// 万、亿结算当前分节；亿还需要将此前的总值一并提升到更高数量级。
				section += number
				if unit == 100000000 {
					total = (total + section) * unit
				} else {
					total += section * unit
				}
				section, number = 0, 0
			} else {
				// “十”“百”等省略首位“一”的写法，按一个对应单位处理。
				if number == 0 {
					number = 1
				}
				section += number * unit
				number = 0
			}
		} else {
			return -1
		}
	}
	return total + section + number
}
