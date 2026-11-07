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

type indexFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}
type cachedIndex struct {
	Files   []indexFile
	Expires time.Time
}

var fileIndexes = struct {
	sync.Mutex
	Entries map[string]cachedIndex
}{Entries: map[string]cachedIndex{}}

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
func indexURL(template, file string) string {
	parts := strings.Split(file, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.ReplaceAll(template, "{file}", strings.Join(parts, "/"))
}
func searchFileIndex(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	result := SearchResponse{Books: []Book{}}
	files, err := loadFileIndex(ctx, client, s.IndexURL)
	if err != nil {
		return result, err
	}
	for _, file := range files {
		if file.Type != "file" || file.Size <= 0 || file.Size > MaxDownloadBytes || strings.Contains(file.Path, `\`) {
			continue
		}
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
