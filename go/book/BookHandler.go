package book

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	url2 "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"wd-reader/go/book/epub/EpubBook"
	"wd-reader/go/book/mobi"
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
		if strings.TrimSpace(na) == "" {
			continue
		}
		mu.Lock()
		if strings.HasPrefix(na, "http") {
			httpUrl = append(httpUrl, na)
		} else if !strings.HasPrefix(na, "http") && (strings.HasSuffix(na, ".txt") || strings.HasSuffix(na, ".epub") || strings.HasSuffix(na, ".mobi") || strings.HasSuffix(na, ".azw") || strings.HasSuffix(na, ".azw3")) && !strings.HasPrefix(na, bookToPath) {
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
				} else if ext == ".mobi" || ext == ".azw3" || ext == ".azw" {
					log.Logger.Info("Processing " + filepath.Base(join))
					reader, err := mobi.NewReader(join)
					if err != nil {
						log.Logger.Error("Parse error:", err)
						continue
					}

					text, err := reader.ExtractText()
					if err != nil {
						log.Logger.Error("Extraction error:", err)
						continue
					}

					if len(text) == 0 {
						log.Logger.Warning("No text extracted (encrypted or empty?)")
						continue
					}

					ext := filepath.Ext(join)
					outPath := join[:len(join)-len(ext)] + ".txt"

					err = os.WriteFile(outPath, append([]byte{0xEF, 0xBB, 0xBF}, []byte(text)...), 0644)
					if err != nil {
						log.Logger.Error("Write error", err)
						continue
					}
					log.Logger.Info("Saved to ", filepath.Base(outPath), " bytes length is:", len(text))
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
