package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"wd-reader/go/book"
	"wd-reader/go/constant"
	"wd-reader/go/log"
	"wd-reader/go/server"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App struct
type App struct {
	application    *application.App
	window         *application.WebviewWindow
	miniMenuMu     sync.Mutex
	miniMenuWindow *application.WebviewWindow
	miniMenuURL    string
	miniMenuState  MiniMenuState
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// OpenFileDialog uses the native Wails 3 file picker.
func (a *App) OpenFileDialog(title string) (string, error) {
	if title == "" {
		title = "文件选择"
	}
	selection, err := a.application.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:          title,
		CanChooseFiles: true,
		Window:         a.window,
		Filters: []application.FileFilter{
			{
				DisplayName: "*.txt;*.epub;*.mobi;*.azw3;*.azw;*.docx;*.dox;*.pdf;*.html;*.htmlz",
				Pattern:     "*.txt;*.epub;*.mobi;*.azw3;*.azw;*.docx;*.dox;*.pdf;*.html;*.htmlz",
			},
		},
		ShowHiddenFiles:      false,
		CanCreateDirectories: false,
		ResolvesAliases:      false,
	}).PromptForSingleSelection()
	// Wails' Windows dialog returns an internal, unexported cancellation error.
	// Treat closing the picker as an empty selection, as on the other platforms.
	if err != nil && err.Error() == "cancelled by user" {
		return "", nil
	}
	return selection, err
}

// LogError keeps frontend errors in the rotating application log.
func (a *App) LogError(message string) { log.GetLogger().Error(message) }

// GetAppPath 获取目前程序运行目录
func (a *App) GetAppPath() string {
	return book.GetAppPath()
}

// GetBooksPath 获取books的目录
func (a *App) GetBooksPath() string {
	return filepath.Join(book.GetAppPath(), constant.BOOK_PATH)
}

func (a *App) GetBookMetadata() ([]book.BookMetadata, error) { return book.ListMetadata() }
func (a *App) RecordBookOpened(name string) error            { return book.RecordBookOpened(name) }
func (a *App) UpdateBookMetadata(id, title string, authors []string, description string) error {
	return book.UpdateMetadata(id, title, authors, description)
}
func (a *App) GetBookCover(id string) (string, error) { return book.CoverData(id) }
func (a *App) RemoveBookCover(id string) error        { return book.SetCover(id, "") }
func (a *App) ChooseBookCover(id string) (bool, error) {
	name, err := a.application.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "选择封面", CanChooseFiles: true, Window: a.window,
		Filters: []application.FileFilter{{DisplayName: "图片", Pattern: "*.jpg;*.jpeg;*.png;*.gif"}},
	}).PromptForSingleSelection()
	if err != nil && err.Error() == "cancelled by user" {
		return false, nil
	}
	if err != nil || name == "" {
		return false, err
	}
	return true, book.SetCover(id, name)
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
	if _, err := book.ImportOriginal(filename, false); err != nil {
		return book.WrapperException(err)
	}
	return "解析成功"
}

// DeleteEpubFile 删除文件
func (a *App) DeleteEpubFile(filename string) string {
	return book.DeleteFile(filename)
}

// GetBookList 获取文件列表
func (a *App) GetBookList() string {
	defer func() {
		if err := recover(); err != nil {
			log.Logger.Fatal(err)
		}
	}()
	return book.GetBookListExtract()
}

// GetChapterListByFileName 传入文件名称获取文件章节或者卷列表
func (a *App) GetChapterListByFileName(_fileName string, splitType string, splitValue string) string {
	defer func() {
		if err := recover(); err != nil {
			log.Logger.Fatal(err)
		}
	}()
	return book.GetChapterListByFileNameExtract(_fileName, splitType, splitValue)
}

// GetChapterContentByChapterName 根据传入文件名和章节名称来获取这一章节的内容
func (a *App) GetChapterContentByChapterName(_fileName string, chapterName string, splitType string, splitValue string) string {
	defer func() {
		if err := recover(); err != nil {
			log.Logger.Fatal(err)
		}
	}()
	return book.GetChapterContentByChpaterNameExtract(_fileName, chapterName, splitType, splitValue)
}

// AddFile 添加文件
func (*App) AddFile(name []string) string {
	var str string
	str = book.ImportFiles(name)
	return str
}

func (a *App) ListOriginalBooks() ([]book.LibraryOriginal, error) { return book.ListOriginals() }
func (a *App) ListMissingBooks() ([]book.MissingBook, error)      { return book.ListMissingBooks() }
func (a *App) RelinkMissingBook(id, target string) error          { return book.RelinkMissingBook(id, target) }
func (a *App) DeleteMissingBook(id string) error                  { return book.DeleteMissingBook(id) }
func (a *App) RestoreOriginalBook(name string, overwrite bool) (string, error) {
	return book.ImportOriginal(name, overwrite)
}
func (a *App) DeleteOriginalBook(name string) error     { return book.DeleteOriginal(name) }
func (a *App) ImportBook(source string) (string, error) { return book.ImportLocal(source) }

// DeleteFile 删除文件
func (*App) DeleteFile(name string) string {
	var str string
	str = book.DeleteFile(name)
	if !strings.HasPrefix(str, constant.ERROR_PREFIX) {
		if err := book.RemoveMetadata(name); err != nil {
			return book.WrapperException(err)
		}
	}
	return str
}

// GetScOne 诗词获取
func (a *App) GetScOne() string {
	defer func() {
		if err := recover(); err != nil {
			log.Logger.Error(err)
		}
	}()
	resp, err := http.Get(constant.WD_SERVER + "/wd/getSc")
	if err != nil {
		log.Logger.Errorf("getscone error,%v", err)
		return ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)
	var data server.Data
	all, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Logger.Errorf("getscone io error,%v", err)
		return ""
	}

	err = json.Unmarshal(all, &data)
	if err != nil {
		log.Logger.Errorf("getscone json error,%v", err)
		return ""
	}
	if data.Success == false {
		marshal, _ := json.Marshal(data)
		log.Logger.Error("line get error ", string(marshal))
		return ""
	}

	m := make(map[string]string)
	m["hitokoto"] = data.Data
	marshal, _ := json.Marshal(m)
	return string(marshal)
}
