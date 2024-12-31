package main

import (
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"path/filepath"
	"wd-reader/go/BookUtils"
	"wd-reader/go/constant"
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
	return BookUtils.GetAppPath()
}

// GetBooksPath 获取books的目录
func (a *App) GetBooksPath() string {
	return filepath.Join(BookUtils.GetAppPath(), constant.BOOK_PATH)
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
	dir := BookUtils.GetAppPath()
	join := filepath.Join(dir, constant.BOOK_PATH, filename)
	err := os.Remove(join)
	if err != nil {
		return BookUtils.WrapperException(err)
	}
	return "ok"
}

// GetBookList 获取文件列表
func (a *App) GetBookList() string {
	return BookUtils.GetBookListExtract()
}

// GetChapterListByFileName 传入文件名称获取文件章节或者卷列表
func (a *App) GetChapterListByFileName(_fileName string) string {
	return BookUtils.GetChapterListByFileNameExtract(_fileName)
}

// GetChapterContentByChapterName 根据传入文件名和章节名称来获取这一章节的内容
func (a *App) GetChapterContentByChapterName(_fileName string, chapterName string) string {
	return BookUtils.GetChapterContentByChpaterNameExtract(_fileName, chapterName)
}

// AddFile 添加文件
func (*App) AddFile(name []string) string {
	var str string
	str = BookUtils.TransferFileFromFileSys(name)
	return str
}

// DeleteFile 删除文件
func (*App) DeleteFile(name string) string {
	var str string
	str = BookUtils.DeleteFile(name)
	return str
}
