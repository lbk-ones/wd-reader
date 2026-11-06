package constant

import (
	"regexp"
	"wd-reader/internal/chapterdetect"
)

const (
	BOOK_PATH    = "books"
	APP_NAME     = "WdReader"
	ERROR_PREFIX = "错误信息:"
	//WD_SERVER    = "http://localhost:9090"
	WD_SERVER = "http://129.226.150.139:10000"
)

var (
	//RegChapter = regexp.MustCompile(`第?([ ,、一二三四五六七八九十零百千万亿\d]+)[章巻、]`)
	// RegChapter 为旧调用方提供统一的章节前缀匹配；最终智能边界需由独立检测模块综合判断。
	RegChapter = regexp.MustCompile(chapterdetect.DefaultPattern)
)
