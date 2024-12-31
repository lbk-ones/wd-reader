package BookUtils

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// DownLoadFile 下载文件
func DownLoadFile(url, filePath string, reMap map[string]string) {
	//filePath := "downloaded_file.zip"  // 下载后保存的文件路径

	// 发送 HTTP GET 请求
	response, err := http.Get(url)
	if err != nil {
		reMap[url] = WrapperException(err)
		fmt.Println("Error making HTTP request:", err)
		return
	}
	defer response.Body.Close()

	// 检查响应状态码是否为 200 OK
	if response.StatusCode != http.StatusOK {
		sprint := fmt.Sprint("Non-200 status code:", response.Status)
		reMap[url] = WrapperExceptionStr(sprint)
		fmt.Println(sprint)
		return
	}

	// 创建文件用于保存下载的内容
	file, err := os.Create(filePath)
	if err != nil {
		reMap[url] = WrapperException(err)
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// 将响应内容复制到文件中
	_, err = io.Copy(file, response.Body)
	if err != nil {
		reMap[url] = WrapperException(err)
		fmt.Println("Error copying content:", err)
		return
	}

	fmt.Println("File downloaded successfully.")
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
