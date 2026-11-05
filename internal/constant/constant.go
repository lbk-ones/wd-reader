package constant

import "regexp"

const (
	BOOK_PATH    = "books"
	APP_NAME     = "WdReader"
	ERROR_PREFIX = "错误信息:"
	//WD_SERVER    = "http://localhost:9090"
	WD_SERVER = "http://129.226.150.139:10000"
)

var (
	//RegChapter = regexp.MustCompile(`第?([ ,、一二三四五六七八九十零百千万亿\d]+)[章巻、]`)
	RegChapter = regexp.MustCompile(`^(?:(?:第\s*)?[零〇一二三四五六七八九十百千万亿\d]+\s*[卷巻章节回篇集]|[零〇一二三四五六七八九十百千万亿\d]+[、：:]|(?i:NO)\s*\d+[、：:. ]|(?i:chapter|book|part)\s+(?i:[ivxlcdm]+|\d+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)\b)`)
)
