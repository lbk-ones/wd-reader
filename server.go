package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"wd-reader/go/BookUtils"
)

const (
	HttpPort = ":10050"
)

// Server struct
type Server struct {
	ctx context.Context
}

// NewServer creates a new Server application struct
func NewServer() *Server {
	return &Server{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *Server) startup(ctx context.Context) {
	a.ctx = ctx
	// 定义处理函数
	//handler := func(w http.ResponseWriter, r *http.Request) {
	//	fmt.Fprintf(w, "Hello, World!")
	//}

	dir, _ := os.Getwd()
	join := filepath.Join(dir, BooksPath)
	fmt.Println("资源目录", join)
	//fileServer := http.FileServer(http.Dir(join))
	// 自定义文件服务器处理函数
	fileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取请求的文件路径
		filePath := filepath.Join(join, r.URL.Path)
		// 检查文件是否存在
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		// 打开文件
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "Error opening file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// 获取文件后缀
		ext := filepath.Ext(filePath)
		// 对于.txt 文件设置 Content-Type 为 text/plain; charset=utf-8
		// 缓存一天
		w.Header().Set("Cache-Control", "public, max-age=5184000")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if ext == ".txt" {
			// 解决乱码的问题
			charset := BookUtils.GetFileCharset(filePath)
			w.Header().Set("Content-Type", "text/plain; charset="+charset)
		}

		reader := bufio.NewReader(file)
		// 读取文件内容
		content, err := io.ReadAll(reader)
		if err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}

		// 处理文件编码问题，假设文件可能是 ANSI 编码，将其转换为 UTF-8
		//if ext == ".txt" {
		//	utf8Content, err := convertToUTF8(content)
		//	if err == nil {
		//		content = utf8Content
		//	}
		//}

		// 输出文件内容
		_, _ = w.Write(content)
	})

	// 注册处理函数
	http.Handle("/", fileServer)

	go func() {
		// 监听端口 8080
		err := http.ListenAndServe(HttpPort, nil)
		fmt.Println("资源服务启动成功 url：", "http://localhost:10050")
		if err != nil {
			fmt.Println("Error starting HTTP server:", err)
		}
	}()

}
