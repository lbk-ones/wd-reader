package book

import (
	"bufio"
	"fmt"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"wd-reader/go/constant"
	"wd-reader/go/log"
)

var (
	mu sync.Mutex
)

// GetDecoder 根据编码方式获取解码器 单独处理国标
func GetDecoder(charset string) *encoding.Decoder {
	replace := strings.ToUpper(strings.ReplaceAll(charset, "-", ""))
	var decoder *encoding.Decoder = nil
	if replace == "GB18030" {
		decoder = simplifiedchinese.GB18030.NewDecoder()
	} else if replace == "GBK" {
		decoder = simplifiedchinese.GBK.NewDecoder()
	} else if replace == "HZGB2312" {
		decoder = simplifiedchinese.HZGB2312.NewDecoder()
	} else if replace == "UTF16LE" {
		decoder = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	} else if replace == "UTF16BE" || replace == "UTF16" {
		decoder = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
	}
	return decoder
}

func WrapperException(err error) string {
	if err != nil {
		return WrapperExceptionStr(err.Error())
	}

	return ""
}
func WrapperExceptionStr(err string) string {
	return fmt.Sprint(constant.ERROR_PREFIX, err)
}

// GetFileCharset 获取文件字符集编码方式
func GetFileCharset(fileName string) string {
	file, err := os.Open(fileName)
	if err != nil {
		return "UTF-8"
	}
	defer file.Close()
	bytes, err := io.ReadAll(io.LimitReader(file, 1024))
	if err != nil || len(bytes) == 0 {
		return "UTF-8"
	}
	if len(bytes) >= 2 {
		if bytes[0] == 0xff && bytes[1] == 0xfe {
			return "UTF-16LE"
		}
		if bytes[0] == 0xfe && bytes[1] == 0xff {
			return "UTF-16BE"
		}
	}

	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(bytes)
	if err != nil || result == nil {
		return "UTF-8"
	}

	return result.Charset
}

// GetScanner 获取文件扫描器
func GetScanner(fileName string, file *os.File) *bufio.Scanner {
	charset := GetFileCharset(fileName)
	decoder := GetDecoder(charset)
	var scanner *bufio.Scanner = nil
	// UTF-8 is read directly; other supported encodings are decoded before scanning.
	if decoder == nil {
		newReader := bufio.NewReader(file)
		scanner = bufio.NewScanner(newReader)
	} else {
		reader := transform.NewReader(file, decoder)
		newReader := bufio.NewReader(reader)
		scanner = bufio.NewScanner(newReader)
	}
	return scanner
}

// GetBookListExtract 获取书的列表
func GetBookListExtract() string {

	if err := EnsureStorage(); err != nil {
		return WrapperException(err)
	}
	bookPath := ParsePath()
	strs := make([]string, 0)
	dirFiles, err := os.ReadDir(bookPath)
	if err != nil {
		return WrapperException(err)
	}
	for _, file := range dirFiles {
		name := file.Name()

		if !file.IsDir() && !strings.HasPrefix(name, ".parse-") && strings.HasSuffix(name, ".txt") {
			mu.Lock()
			// 只处理 txt 和 epub
			strs = append(strs, name)
			mu.Unlock()
		}
	}
	return strings.Join(strs, "\n")
}

// GetChapterListByFileNameExtract returns only chapters with readable bodies.
func GetChapterListByFileNameExtract(fileName, splitType, splitValue string) string {
	chapters, err := readBookChapters(fileName, splitType, splitValue)
	if err != nil {
		return WrapperException(err)
	}
	names := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		names = append(names, chapter.Name)
	}
	return strings.Join(names, "\n")
}

// GetChapterContentByChpaterNameExtract resolves the same unique names as the directory.
func GetChapterContentByChpaterNameExtract(fileName, chapterName, splitType, splitValue string) string {
	chapters, err := readBookChapters(fileName, splitType, splitValue)
	if err != nil {
		return WrapperException(err)
	}
	for _, chapter := range chapters {
		if chapter.Name == chapterName {
			return strings.Join(chapter.Body, "\n")
		}
	}
	return WrapperExceptionStr("章节不存在或没有正文，请重新选择章节")
}

// TransferFileFromFileSys  下载文件
func TransferFileFromFileSys(name []string) string { return ImportFiles(name) }

// DeleteFile 删除文件
func DeleteFile(name string) string {
	if !validBookName(name) {
		return WrapperExceptionStr("invalid book name")
	}
	join := filepath.Join(ParsePath(), name)
	log.Logger.Info("delete file " + join)
	err := os.Remove(join)

	return WrapperException(err)

}
