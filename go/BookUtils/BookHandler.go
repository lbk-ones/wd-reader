package BookUtils

import (
	"bufio"
	"fmt"
	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"os"
	"strings"
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
	return fmt.Sprint("错误信息:", err)
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
