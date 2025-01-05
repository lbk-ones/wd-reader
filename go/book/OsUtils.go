package book

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"wd-reader/go/constant"
)

// GetAppPath 获取程序目录
func GetAppPath() string {
	if IsMac() || IsLinux() {
		homeDir, _ := os.UserHomeDir()
		return homeDir
	}
	dir, _ := os.Getwd()
	return dir
}

// IsWindows 判断是否是windows
func IsWindows() bool {
	return GetGods() == "windows"
}

// IsLinux 判断是否是 linux
func IsLinux() bool {
	return GetGods() == "linux"
}

// IsMac 判断是否是 mac
func IsMac() bool {
	return GetGods() == "darwin"
}

func GetGods() string {

	osName := runtime.GOOS

	return osName

}

// CheckBooksPath  检查目录 没有则新增
func CheckBooksPath() {
	join := filepath.Join(GetAppPath(), constant.BOOK_PATH)
	_, err := os.Stat(join)
	if err != nil {
		if os.IsNotExist(err) {
			// 0777
			err := os.MkdirAll(join, fs.ModePerm)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
