package epubexport

import (
	"html"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

// DecodeTXT decodes the encodings used by the Chinese book sources.
func DecodeTXT(data []byte) (string, error) {
	if len(data) >= 2 && (data[0] == 0xff && data[1] == 0xfe || data[0] == 0xfe && data[1] == 0xff) {
		return unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder().String(string(data))
	}
	if utf8.Valid(data) {
		return strings.TrimPrefix(string(data), "\ufeff"), nil
	}
	return simplifiedchinese.GB18030.NewDecoder().String(string(data))
}

// UnescapeHTML also handles entities that were escaped more than once.
func UnescapeHTML(text string) string {
	for {
		decoded := html.UnescapeString(text)
		if decoded == text {
			return text
		}
		text = decoded
	}
}
