package book

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	url2 "net/url"
	"os"
	"path/filepath"
	"strings"
	"wd-reader/go/book/epub/EpubBook"
	"wd-reader/go/constant"
)

// GetDecoder 根据编码方式获取解码器 单独处理国标
func GetDecoder(charset string) *encoding.Decoder {
	replace := strings.Replace(charset, "-", "", -1)
	var decoder *encoding.Decoder = nil
	if replace == "GB18030" {
		decoder = simplifiedchinese.GB18030.NewDecoder()
	} else if replace == "GBK" {
		decoder = simplifiedchinese.GBK.NewDecoder()
	} else if replace == "HZGB2312" {
		decoder = simplifiedchinese.HZGB2312.NewDecoder()
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
	bytes := make([]byte, 1024)
	file, _ := os.Open(fileName)
	_, err := file.Read(bytes)
	if err != nil {
		fmt.Println("error:", err)
	}
	defer file.Close()

	detector := chardet.NewTextDetector()
	result, _ := detector.DetectBest(bytes)

	return result.Charset
}

// GetScanner 获取文件扫描器
func GetScanner(fileName string, file *os.File) *bufio.Scanner {
	charset := GetFileCharset(fileName)
	decoder := GetDecoder(charset)
	var scanner *bufio.Scanner = nil
	// 非国标编码 可能是 utf-8 utf-16 === 这些就不管了
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
	CheckBooksPath()
	path := GetAppPath()
	bookPath := filepath.Join(path, constant.BOOK_PATH)
	strs := make([]string, 0)
	dirFiles, err := os.ReadDir(bookPath)
	if err != nil {
		return WrapperException(err)
	}
	for _, file := range dirFiles {
		name := file.Name()

		if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".epub") {
			// 只处理 txt 和 epub
			strs = append(strs, name)
		}
	}
	return strings.Join(strs, "\n")
}

// GetChapterListByFileNameExtract  获取章节列表 抽离
func GetChapterListByFileNameExtract(_fileName string) string {
	path := GetAppPath()
	fileName := filepath.Join(path, constant.BOOK_PATH, _fileName)
	if strings.HasSuffix(fileName, ".txt") {
		strs := make([]string, 0)
		// 判断文件是否存在
		_, err := os.Stat(fileName)
		if os.IsNotExist(err) {
			return WrapperException(err)
		}
		file, err := os.Open(fileName)
		if err != nil {
			return WrapperException(err)
		}
		defer func(file *os.File) {
			err2 := file.Close()
			if err2 != nil {
				fmt.Println(err2)
			}
		}(file)
		scanner := GetScanner(fileName, file)
		unique := make(map[string]struct{})
		var index int = 0
		for scanner.Scan() {
			text := scanner.Text()
			//prefix := strings.HasPrefix(text, "  ")
			//if prefix {
			//	continue
			//}
			line := strings.TrimSpace(text)
			if line == "" {
				continue
			}
			index++
			findString := constant.RegChapter.FindString(line)
			if findString != "" {
				replaceNonSpace := strings.ReplaceAll(line, " ", "")
				if _, exists := unique[replaceNonSpace]; !exists {
					unique[replaceNonSpace] = struct{}{}
					strs = append(strs, line)
				}
			}
		}
		return strings.Join(strs, "\n")
	}
	return ""
}

// GetChapterContentByChpaterNameExtract  获取章节内容
func GetChapterContentByChpaterNameExtract(_fileName string, chapterName string) string {
	path := GetAppPath()
	fileName := filepath.Join(path, constant.BOOK_PATH, _fileName)
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return WrapperException(err)
	}
	file, err := os.Open(fileName)
	if err != nil {
		return WrapperException(err)
	}
	if chapterName == "" {
		return WrapperExceptionStr(chapterName + " is not a chapter name")
	}
	defer file.Close()
	scanner := GetScanner(fileName, file)
	var strs []string
	var start bool
	lastLineNoAllSpace := strings.Replace(chapterName, " ", "", -1)
	chapterNameNoSpace := strings.TrimSpace(chapterName)
	var index int = 0
	var lstIndex int = 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineNoAllSpace := strings.Replace(line, " ", "", -1)
		index++
		// 开始记录 跳过标题
		if line == chapterNameNoSpace && !start {
			start = true
			lstIndex = index
			continue
		}
		// 碰到下一个结束标识符
		// 因为有些文本有重复章节名称 且除了空格都一样 所以这里要 去掉空格之后 再去比较 不等于才退出
		findString := constant.RegChapter.FindString(line)
		// 两行临近这种也不记录 可能是错误的文本校准 只有文本相同了才跳过
		// 不跳过
		if start {
			if findString != "" {
				// 第二行重复跳出
				b := lstIndex+1 == index && lastLineNoAllSpace == lineNoAllSpace
				if b {
					continue
				} else {
					break
				}
				// 第二行不重复 直接跳出
				//b2 := index > lstIndex+1
				//if b2 {
				//	break
				//}
			}
			strs = append(strs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return WrapperExceptionStr(fmt.Sprintln(fmt.Errorf("error while scanning file: %w", err)))
	}
	return strings.Join(strs, "\n")
}

// TransferFileFromFileSys  下载文件
func TransferFileFromFileSys(name []string) string {

	var resJsonMap = make(map[string]string)

	path := GetAppPath()
	bookToPath := filepath.Join(path, constant.BOOK_PATH)
	var httpUrl []string
	var fileUrl []string
	for _, na := range name {
		if strings.HasPrefix(na, "http") {
			httpUrl = append(httpUrl, na)
		} else if !strings.HasPrefix(na, "http") && (strings.HasSuffix(na, ".txt") || strings.HasSuffix(na, ".epub")) && !strings.HasPrefix(na, bookToPath) {
			fileUrl = append(fileUrl, na)
		} else {
			resJsonMap[na] = "路径不合法"
		}
	}

	if len(httpUrl) > 0 {
		for _, ht := range httpUrl {
			base := filepath.Base(ht)
			join := filepath.Join(bookToPath, base)
			join, _ = url2.QueryUnescape(join)
			DownLoadFile(ht, resJsonMap)
		}
	}

	if len(fileUrl) > 0 {
		for _, ht := range fileUrl {
			base := filepath.Base(ht)
			ext := filepath.Ext(ht)
			join := filepath.Join(bookToPath, base)
			joinOutputPath := strings.Replace(join, ".epub", ".txt", -1)
			if base != "." {
				CopyFileToOtherPath(ht, join, resJsonMap)
				if ext == ".epub" {
					epub, err := EpubBook.ParseEpub(join, joinOutputPath)
					if err != nil {
						resJsonMap[ht] = fmt.Sprint(err)
						continue
					}
					err = epub.WriteEpub()
					if err != nil {
						resJsonMap[ht] = fmt.Sprint(err)
						continue
					}
					//s := ParseEpubToTxt(base)
					//if strings.HasPrefix(s, constant.ERROR_PREFIX) {
					//	resJsonMap[ht] = "epub文件解析失败," + s
					//}
					DeleteFile(base)
				}
			} else {
				resJsonMap[ht] = "路径不能为空呢"
			}
		}
	}

	//if len(fileUrl) == 0 && len(httpUrl) == 0 {
	//	for _, na := range name {
	//		resJsonMap[na] = "路径不合法"
	//	}
	//}

	marshal, err := json.Marshal(resJsonMap)
	if err != nil {
		return WrapperExceptionStr(fmt.Sprintln(err))
	}
	return string(marshal)
}

// DeleteFile 删除文件
func DeleteFile(name string) string {

	err := os.Remove(filepath.Join(GetAppPath(), constant.BOOK_PATH, name))

	return WrapperException(err)

}
