package constant

import "regexp"

const (
	BOOK_PATH = "books"
)

var (
	RegChapter = regexp.MustCompile(`第([ ,、一二三四五六七八九十零百千万亿\d]+)[章巻]`)
)
