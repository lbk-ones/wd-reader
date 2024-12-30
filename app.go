package main

import (
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"wd-reader/go/BookUtils"
)

const (
	BooksPath = "books"
)

var (
	RegChapter = regexp.MustCompile(`第([ ,、一二三四五六七八九十零百千万亿\d]+)[章巻]`)
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// OpenFileDialog Deprecated 目前这个好像有BUG用不起来 就很烦
func (a *App) OpenFileDialog() string {

	fmt.Println("开始调用")
	dialog, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "文件选择",
		DefaultDirectory: "./",
		DefaultFilename:  "",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "*.txt",
				Pattern:     "*.txt",
			},
		},
		ShowHiddenFiles:            false,
		CanCreateDirectories:       false,
		ResolvesAliases:            false,
		TreatPackagesAsDirectories: false,
	})
	if err != nil {
		fmt.Println(fmt.Errorf("failed to open file dialog: %v", err))
	}
	fmt.Println("调用返回" + dialog)
	return dialog
}

// GetAppPath 获取目前程序运行目录
func (a *App) GetAppPath() string {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("获取当前工作目录时出错:", err)
		return ""
	}
	fmt.Println("当前工作目录:", wd)
	return wd
}

// GetVersion 获取版本
func (a *App) GetVersion() string {
	return version
}
func (a *App) GetServerUrl() string {

	return `http://localhost` + HttpPort + "/"
}

// ParseEpubToTxt 解析epub文件
func (a *App) ParseEpubToTxt(filename string) string {
	return BookUtils.ParseEpubToTxt(filename)
}

// DeleteEpubFile 删除文件
func (a *App) DeleteEpubFile(filename string) string {
	dir, _ := os.Getwd()
	join := filepath.Join(dir, BooksPath, filename)
	err := os.Remove(join)
	if err != nil {
		return BookUtils.WrapperException(err)
	}
	return "ok"
}

// GetBookList 获取文件列表
func (a *App) GetBookList() string {
	path, _ := os.Getwd()
	bookPath := filepath.Join(path, BooksPath)

	fileInfo, err := os.Stat(bookPath + string(filepath.Separator))
	if os.IsNotExist(err) {
		return BookUtils.WrapperExceptionStr(bookPath + "::目录不存在::" + err.Error())
	}
	if !fileInfo.IsDir() {
		return BookUtils.WrapperExceptionStr(fmt.Sprint(bookPath + " is not a directory"))
	}

	strs := make([]string, 0)
	dirFiles, err := os.ReadDir(bookPath)
	if err != nil {
		return BookUtils.WrapperException(err)
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

// GetChapterListByFileName 传入文件名称获取文件章节或者卷列表
func (a *App) GetChapterListByFileName(_fileName string) string {
	path, _ := os.Getwd()
	fileName := filepath.Join(path, BooksPath, _fileName)
	if strings.HasSuffix(fileName, ".txt") {
		strs := make([]string, 0)
		// 判断文件是否存在
		_, err := os.Stat(fileName)
		if os.IsNotExist(err) {
			return BookUtils.WrapperException(err)
		}
		file, err := os.Open(fileName)
		if err != nil {
			return BookUtils.WrapperException(err)
		}
		defer file.Close()
		scanner := BookUtils.GetScanner(fileName, file)
		unique := make(map[string]struct{})
		var index int = 0
		var lstIndex int = 0
		for scanner.Scan() {
			text := scanner.Text()
			index++
			prefix := strings.HasPrefix(text, "  ")
			if prefix {
				continue
			}
			line := strings.TrimSpace(text)
			if line == "" {
				continue
			}
			lineLen := len(line)
			findString := RegChapter.FindString(line)
			b := lstIndex+1 == index
			if findString != "" && !b && lineLen < 50 {
				replaceNonSpace := strings.Replace(line, " ", "", -1)
				if _, exists := unique[replaceNonSpace]; !exists {
					unique[replaceNonSpace] = struct{}{}
					strs = append(strs, line)
					lstIndex = index
				}
			}
		}
		return strings.Join(strs, "\n")
	}
	return ""
}

// GetChapterContentByChapterName 根据传入文件名和章节名称来获取这一章节的内容
func (a *App) GetChapterContentByChapterName(_fileName string, chapterName string) string {
	path, _ := os.Getwd()
	fileName := filepath.Join(path, BooksPath, _fileName)
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return BookUtils.WrapperException(err)
	}
	file, err := os.Open(fileName)
	if err != nil {
		return BookUtils.WrapperException(err)
	}
	if chapterName == "" {
		return BookUtils.WrapperExceptionStr(fileName + " is not a chapter name")
	}
	defer file.Close()
	scanner := BookUtils.GetScanner(fileName, file)
	var strs []string
	var start bool
	lastLineNoAllSpace := strings.Replace(chapterName, " ", "", -1)
	chapterNameNoSpace := strings.TrimSpace(chapterName)
	var index int = 0
	var lstIndex int = 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNoAllSpace := strings.Replace(line, " ", "", -1)
		index++
		// 开始记录
		if line == chapterNameNoSpace && !start {
			start = true
			lstIndex = index
			continue
		}
		// 碰到下一个结束标识符
		// 因为有些文本有重复章节名称 且除了空格都一样 所以这里要 去掉空格之后 再去比较 不等于才退出
		findString := RegChapter.FindString(line)
		// 两行临近这种也不记录 可能是错误的文本校准
		b := findString != "" && lstIndex+1 != index
		if start && b && lineNoAllSpace != lastLineNoAllSpace {
			break
		}
		if start {
			strs = append(strs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return BookUtils.WrapperExceptionStr(fmt.Sprintln(fmt.Errorf("error while scanning file: %w", err)))
	}
	return strings.Join(strs, "\n")
}
