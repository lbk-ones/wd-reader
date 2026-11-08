package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// indexFile 对应公开 JSON 文件目录中的一项；当前适配 path、size、type 结构。
type indexFile struct {
	// 目录内使用正斜杠的相对路径。
	Path string `json:"path"`
	// 文件字节数，用于排除空文件或过大文件。
	Size int64 `json:"size"`
	// 条目类型，仅 file 条目参与搜索。
	Type string `json:"type"`
}

// cachedIndex 保存一次目录查询的只读结果及过期时间。
type cachedIndex struct {
	// 缓存的目录切片，调用方只读取，不原地修改。
	Files []indexFile
	// 缓存失效时间；超过此时间需要重新请求目录。
	Expires time.Time
}

// fileIndexes 按索引 URL 共享缓存；互斥锁只保护缓存表，网络请求不持锁。
var fileIndexes = struct {
	// 保护缓存表的读取、清理和替换。
	sync.Mutex
	// 目录地址到只读缓存结果的映射。
	Entries map[string]cachedIndex
}{Entries: map[string]cachedIndex{}}

// loadFileIndex 读取并缓存 JSON 文件目录 5 分钟；网络请求在锁外执行，缓存最多保留 30 个地址。
func loadFileIndex(ctx context.Context, client *http.Client, target string) ([]indexFile, error) {
	fileIndexes.Lock()
	cached, ok := fileIndexes.Entries[target]
	fileIndexes.Unlock()
	if ok && time.Now().Before(cached.Expires) {
		return cached.Files, nil
	}
	data, err := fetch(ctx, client, target)
	if err != nil {
		return nil, err
	}
	var files []indexFile
	if err = json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("invalid file index: %w", err)
	}
	if files == nil {
		return nil, fmt.Errorf("file index must be an array")
	}
	fileIndexes.Lock()
	// 先清除过期项；仍达到容量上限时重置缓存，限制进程内存增长。
	if len(fileIndexes.Entries) >= 30 {
		for key, item := range fileIndexes.Entries {
			if time.Now().After(item.Expires) {
				delete(fileIndexes.Entries, key)
			}
		}
		if len(fileIndexes.Entries) >= 30 {
			fileIndexes.Entries = map[string]cachedIndex{}
		}
	}
	fileIndexes.Entries[target] = cachedIndex{Files: files, Expires: time.Now().Add(5 * time.Minute)}
	fileIndexes.Unlock()
	return files, nil
}

// indexURL 逐段转义目录文件路径后替换 {file}，保留目录分隔符而转义文件名中的特殊字符。
func indexURL(template, file string) string {
	parts := strings.Split(file, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.ReplaceAll(template, "{file}", strings.Join(parts, "/"))
}

// ListIndexedBooks lists supported files from a configured file index.
func ListIndexedBooks(ctx context.Context, client *http.Client, s Source) ([]Book, error) {
	result, err := indexedBooks(ctx, client, s, "")
	return result.Books, err
}

func indexedBooks(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	result := SearchResponse{Books: []Book{}}
	files, err := loadFileIndex(ctx, client, s.IndexURL)
	if err != nil {
		return result, err
	}
	for _, file := range files {
		if file.Type != "file" || file.Size <= 0 || file.Size > MaxDownloadBytes || strings.Contains(file.Path, `\`) {
			continue
		}
		// 拒绝父路径、当前路径和空路径段，防止目录数据拼出意外的下载地址。
		unsafe := false
		for _, part := range strings.Split(file.Path, "/") {
			if part == ".." || part == "." || part == "" {
				unsafe = true
			}
		}
		if unsafe {
			continue
		}
		ext := strings.ToLower(path.Ext(file.Path))
		if !supportedExtension(ext) {
			continue
		}
		name := path.Base(file.Path)
		title := strings.TrimSuffix(name, path.Ext(name))
		if !matchesTitle(title, query) {
			continue
		}
		target := indexURL(s.DownloadURL, file.Path)
		bookURL := indexURL(s.BookURL, file.Path)
		if !IsWebURL(target) || !IsWebURL(bookURL) {
			continue
		}
		// 文件目录没有作者元数据，此处只使用配置提示，不能证明文件正文或版本。
		author := ""
		for bookName, writer := range s.AuthorHints {
			if clean(bookName) == clean(title) {
				author = writer
				break
			}
		}
		matchType := "related"
		if clean(title) == clean(query) {
			matchType = "exact"
		}
		result.Books = append(result.Books, Book{ID: file.Path, Title: title, Author: author, URL: bookURL, SourceID: s.ID, SourceName: s.Name, MatchType: matchType, Files: []File{{URL: target, Name: SafeFileName(name, ext), Format: strings.ToUpper(ext[1:]), Size: file.Size}}})
	}
	return result, nil
}

// searchFileIndex 在公开文件目录内按文件名匹配书籍，最多返回 8 项。
func searchFileIndex(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	result, err := indexedBooks(ctx, client, s, query)
	if err != nil {
		return result, err
	}
	// 同名结果优先，其余按书名排序，最后截取最多 8 项。
	sort.SliceStable(result.Books, func(i, j int) bool {
		a, b := result.Books[i], result.Books[j]
		if a.MatchType != b.MatchType {
			return a.MatchType == "exact"
		}
		return a.Title < b.Title
	})
	if len(result.Books) > 8 {
		result.Books = result.Books[:8]
	}
	return result, nil
}
