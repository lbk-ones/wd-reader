package book

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	url2 "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"wd-reader/go/book/epub/EpubBook"
	"wd-reader/go/constant"
	"wd-reader/go/log"
	"wd-reader/go/server"
	"wd-reader/go/utils"
)

var (
	mu sync.Mutex
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
			mu.Lock()
			// 只处理 txt 和 epub
			strs = append(strs, name)
			mu.Unlock()
		}
	}
	return strings.Join(strs, "\n")
}

// GetChapterListByFileNameExtract  获取章节列表 抽离
func GetChapterListByFileNameExtract(_fileName string) string {
	path := GetAppPath()
	fileName := filepath.Join(path, constant.BOOK_PATH, _fileName)
	if strings.HasSuffix(fileName, ".txt") {
		var strs = make([]string, 0)
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
					mu.Lock()
					strs = append(strs, line)
					mu.Unlock()
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
			mu.Lock()
			strs = append(strs, line)
			mu.Unlock()
		}
	}
	if err := scanner.Err(); err != nil {
		return WrapperExceptionStr(fmt.Sprintln(fmt.Errorf("error while scanning file: %w", err)))
	}
	return strings.Join(strs, "\n")
}

// TransferFileFromFileSys  下载文件
func TransferFileFromFileSys(name []string) string {

	log.Logger.Info("begin add file to app...")
	var resJsonMap = make(map[string]string)

	path := GetAppPath()
	bookToPath := filepath.Join(path, constant.BOOK_PATH)
	log.Logger.Info("book path is ", bookToPath)
	var httpUrl []string
	var fileUrl []string
	var otherUrl []string
	array := []string{".mobi", ".azw3", ".azw", ".docx", ".doc", ".pdf", ".html", ".htmlz"}
	log.Logger.Info("exclude txt,epub  only allow other suffix ", strings.Join(array, ","))
	for _, na := range name {

		mu.Lock()
		if strings.HasPrefix(na, "http") {
			httpUrl = append(httpUrl, na)
		} else if !strings.HasPrefix(na, "http") && (strings.HasSuffix(na, ".txt") || strings.HasSuffix(na, ".epub")) && !strings.HasPrefix(na, bookToPath) {
			fileUrl = append(fileUrl, na)
		} else if utils.ContainsSuffix(array, na) {
			log.Logger.Info("detect file type " + filepath.Ext(na))
			otherUrl = append(otherUrl, na)
		} else {
			resJsonMap[na] = "路径不合法"
		}
		mu.Unlock()
	}

	if len(httpUrl) > 0 {
		log.Logger.Info("httpUrl length is ", len(httpUrl))
		for _, ht := range httpUrl {
			base := filepath.Base(ht)
			join := filepath.Join(bookToPath, base)
			join, _ = url2.QueryUnescape(join)
			DownLoadFile(ht, resJsonMap)
		}
	}

	if len(fileUrl) > 0 {
		log.Logger.Info("fileUrl length is ", len(fileUrl))
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

	// parse other file
	if len(otherUrl) > 0 {
		log.Logger.Info("begin handler other file")
		log.Logger.Info("otherUrl length is ", len(otherUrl))
		file := parseOtherFile(otherUrl, resJsonMap)
		log.Logger.Info("end handler other file" + file)
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
	join := filepath.Join(GetAppPath(), constant.BOOK_PATH, name)
	log.Logger.Info("delete file " + join)
	err := os.Remove(join)

	return WrapperException(err)

}

// 解析其他类型文件
func parseOtherFile(name []string, resJsonMap map[string]string) string {

	path := GetAppPath()
	bookToPath := filepath.Join(path, constant.BOOK_PATH)
	var updList []string
	for _, fn := range name {
		if !(strings.HasSuffix(fn, ".epub") && strings.HasSuffix(fn, ".txt")) {
			mu.Lock()
			updList = append(updList, fn)
			mu.Unlock()
		}
	}
	nameVsDownUrlMap := make(map[string]string)
	// online parse not epub file
	var epubList []string
	for _, fname := range updList {
		filename := filepath.Base(fname)
		param := make(map[string]string)
		param["filename"] = filename
		open, err := os.Open(fname)
		if err != nil {
			resJsonMap[fname] = WrapperExceptionStr(fmt.Sprintln(err))
			continue
		}
		all, err := io.ReadAll(open)
		if err != nil {
			resJsonMap[fname] = WrapperExceptionStr(fmt.Sprintln(err))
			continue
		}
		stream, err := server.PostOctetStream(param, all)
		if err != nil {
			str := WrapperExceptionStr(fmt.Sprintln(err))
			log.Logger.Info("download error:" + str)
			resJsonMap[fname] = str
			continue
		}
		if stream.Success == true {
			url := stream.Data.FileUrl
			s := stream.Data.DownloadUrl + url
			s3, _ := url2.QueryUnescape(s)
			// download
			DownLoadFile(s3, resJsonMap)
			if resJsonMap[s3] == "" {
				join := filepath.Join(bookToPath, filepath.Base(s3))
				nameVsDownUrlMap[join] = fname
				mu.Lock()
				epubList = append(epubList, join)
				mu.Unlock()
			}
		}

	}

	// parse epub
	if len(epubList) > 0 {
		for _, newName := range epubList {
			originName := nameVsDownUrlMap[newName]
			base := filepath.Base(originName)
			originExt := filepath.Ext(originName)
			join := filepath.Join(bookToPath, base)
			joinOutputPath := strings.Replace(join, originExt, ".txt", -1)
			epub, err := EpubBook.ParseEpub(newName, joinOutputPath)
			if err != nil {
				resJsonMap[newName] = fmt.Sprint(err)
			}
			err = epub.WriteEpub()
			if err != nil {
				resJsonMap[originName] = fmt.Sprint(err)
				continue
			}
			// delete new epub file
			DeleteFile(filepath.Base(newName))
		}
	} else {
		marshal, _ := json.Marshal(resJsonMap)
		return WrapperExceptionStr(string(marshal))
	}
	return "ok"

}
