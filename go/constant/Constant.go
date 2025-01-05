package constant

import "regexp"

const (
	BOOK_PATH    = "books"
	APP_NAME     = "WdReader"
	ERROR_PREFIX = "错误信息:"
)

var (
	//RegChapter = regexp.MustCompile(`第?([ ,、一二三四五六七八九十零百千万亿\d]+)[章巻、]`)
	RegChapter = regexp.MustCompile(`^(第|NO)?([A-Za-z,、一二三四五六七八九十零百千万亿\d]+[卷章、：:]+)`)
)
