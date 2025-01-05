package book

import (
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"os"
	"path"
	"strings"
	"wd-reader/go/constant"
)

// DownLoadFile 下载文件
func DownLoadFile(url string, reMap map[string]string) {
	// 发送 HTTP GET 请求
	response, err := http.Get(url)
	url, _ = url2.QueryUnescape(url)
	if err != nil {
		reMap[url] = WrapperException(err)
		fmt.Println("Error making HTTP request:", err)
		return
	}

	// 检查响应状态码是否为 200 OK
	if response.StatusCode != http.StatusOK {
		sprint := fmt.Sprint("Non-200 status code:", response.Status)
		reMap[url] = WrapperExceptionStr(sprint)
		fmt.Println(sprint)
		return
	}
	name, err := getFileName(response, url)
	if err != nil {
		reMap[url] = WrapperException(err)
		return
	}

	CheckBooksPath()
	appPath := GetAppPath()
	outPath := path.Join(appPath, constant.BOOK_PATH, name)
	// 创建文件用于保存下载的内容
	file, err := os.Create(outPath)
	if err != nil {
		reMap[url] = WrapperException(err)
		fmt.Println("Error creating file:", err)
		return
	}

	defer func() {
		err2 := response.Body.Close()
		if err2 != nil {
			fmt.Println("Error closing response body:", err2)
		}
		err2 = file.Close()
		if err2 != nil {
			fmt.Println("Error closing file:", err2)
		}
	}()

	// 将响应内容复制到文件中
	_, err = io.Copy(file, response.Body)
	if err != nil {
		reMap[url] = WrapperException(err)
		fmt.Println("Error copying content:", err)
		return
	}

	fmt.Println("File downloaded successfully.")
}

// urlstr is Unescape
func getFileName(resp *http.Response, urlstr string) (string, error) {
	// from Content-Disposition get filename
	var filename string
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		// find filename
		index := strings.Index(contentDisposition, "filename=")
		if index >= 0 {
			filename = contentDisposition[index+len("filename="):]
			// 处理可能的引号
			if len(filename) > 0 && filename[0] == '"' {
				filename = strings.Trim(filename, `"`)
			}
			if filename != "" {
				return filename, nil
			}
		}
	}
	ext := path.Ext(urlstr)
	if ext != "" {
		return path.Base(urlstr), nil
	}

	parsedURL, err := url2.Parse(urlstr)
	if err != nil {
		return "", err
	}
	filename = path.Base(parsedURL.Path)

	// remove query parameter
	questionMarkIndex := strings.Index(filename, "?")
	if questionMarkIndex != -1 {
		filename = filename[:questionMarkIndex]
	}
	return filename, nil
}

// CopyFileToOtherPath 文件复制
func CopyFileToOtherPath(fromPath, toPath string, reMap map[string]string) {

	open, err := os.Open(fromPath)
	if os.IsNotExist(err) {
		reMap[fromPath] = WrapperException(err)
		return
	}

	defer open.Close()

	create, _ := os.Create(toPath)
	defer create.Close()

	_, err = io.Copy(create, open)
	if err != nil {
		reMap[fromPath] = WrapperException(err)
		return
	}

}
