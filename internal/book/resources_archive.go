package book

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"wd-reader/internal/constant"
)

const (
	resourceManifestName = "wd-reader-resources.json"
	resourceCacheName    = "frontend-cache.json"
	resourceCacheLimit   = 16 << 20
	resourceArchiveLimit = uint64(32 << 30)
)

type FrontendCache struct {
	LocalStorage   map[string]string `json:"localStorage"`
	SessionStorage map[string]string `json:"sessionStorage"`
}

type ResourceProgress struct {
	Stage     string `json:"stage"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

type resourceManifest struct {
	Format     string    `json:"format"`
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
}

func resourcesPath() string { return filepath.Join(GetAppPath(), constant.BOOK_PATH) }

func reportResources(report func(ResourceProgress), stage string, completed, total int) {
	if report != nil {
		report(ResourceProgress{Stage: stage, Completed: completed, Total: total})
	}
}

type resourceReader struct {
	ctx context.Context
	io.Reader
}

func (r resourceReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(data)
}

func resourceEntries(root string) ([]string, error) {
	var entries []string
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("资源目录包含不支持的链接或特殊文件: %s", name)
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if rel != "." {
			entries = append(entries, filepath.ToSlash(rel))
		}
		return nil
	})
	return entries, err
}

// ExportResources writes the entire books tree and browser storage to a portable ZIP.
func ExportResources(ctx context.Context, destination string, cache FrontendCache, report func(ResourceProgress)) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	destination, err := filepath.Abs(destination)
	if err != nil {
		return 0, err
	}
	cacheData, err := json.Marshal(cache)
	if err != nil || len(cacheData) > resourceCacheLimit {
		return 0, fmt.Errorf("前端缓存超过导出限制")
	}
	if _, err := ListMetadata(); err != nil {
		return 0, err
	}
	importMu.Lock()
	defer importMu.Unlock()
	storageMu.Lock()
	defer storageMu.Unlock()
	metadataMu.Lock()
	defer metadataMu.Unlock()

	books, err := filepath.EvalSymlinks(resourcesPath())
	if err != nil {
		return 0, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return 0, err
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	if rel, err := filepath.Rel(books, destination); err == nil && filepath.IsLocal(rel) {
		return 0, fmt.Errorf("请将导出文件保存在 books 目录之外")
	}
	entries, err := resourceEntries(books)
	if err != nil {
		return 0, err
	}
	root, err := os.OpenRoot(books)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	out, err := os.CreateTemp(parent, ".wd-reader-export-*.zip")
	if err != nil {
		return 0, err
	}
	defer os.Remove(out.Name())
	defer out.Close()
	archive := zip.NewWriter(out)
	defer archive.Close()
	manifest, _ := json.Marshal(resourceManifest{Format: "wd-reader-resources", Version: 1, ExportedAt: time.Now().UTC()})
	for name, data := range map[string][]byte{resourceManifestName: manifest, resourceCacheName: cacheData} {
		writer, err := archive.Create(name)
		if err != nil {
			return 0, err
		}
		if _, err := writer.Write(data); err != nil {
			return 0, err
		}
	}
	count := 0
	for i, name := range entries {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		info, err := root.Stat(filepath.FromSlash(name))
		if err != nil {
			return 0, err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return 0, err
		}
		header.Name = "books/" + name
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return 0, err
		}
		if !info.IsDir() {
			source, err := root.Open(filepath.FromSlash(name))
			if err != nil {
				return 0, err
			}
			_, err = io.Copy(writer, resourceReader{ctx, source})
			source.Close()
			if err != nil {
				return 0, err
			}
			count++
		}
		reportResources(report, "exporting", i+1, len(entries))
	}
	if err := archive.Close(); err != nil {
		return 0, err
	}
	if err := out.Sync(); err != nil {
		return 0, err
	}
	if err := out.Close(); err != nil {
		return 0, err
	}
	return count, os.Rename(out.Name(), destination)
}

func validResourcePath(name string) bool {
	return name != "" && name != "." && path.Clean(name) == name &&
		!strings.ContainsAny(name, "\\:\x00") && filepath.IsLocal(filepath.FromSlash(name))
}

func readResourceJSON(file *zip.File, limit int64, target any) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("资源包中的 %s 超过大小限制", file.Name)
	}
	return json.Unmarshal(data, target)
}

// extractResourceArchive validates every entry before extracting into an isolated directory.
func extractResourceArchive(ctx context.Context, filename, stage string, report func(ResourceProgress)) (FrontendCache, error) {
	cache := FrontendCache{}
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return cache, fmt.Errorf("无法打开资源包: %w", err)
	}
	defer archive.Close()
	if len(archive.File) > 100000 {
		return cache, fmt.Errorf("资源包包含过多文件")
	}
	seen := map[string]bool{}
	var manifestFile, cacheFile *zip.File
	var total uint64
	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, "/")
		key := strings.ToLower(name)
		if !validResourcePath(name) || seen[key] ||
			(!file.Mode().IsRegular() && !file.Mode().IsDir()) ||
			(name != resourceManifestName && name != resourceCacheName && name != "books" && !strings.HasPrefix(name, "books/")) {
			return cache, fmt.Errorf("资源包包含无效或重复的路径: %s", file.Name)
		}
		seen[key] = true
		if file.UncompressedSize64 > resourceArchiveLimit-total {
			return cache, fmt.Errorf("资源包解压后超过大小限制")
		}
		total += file.UncompressedSize64
		switch name {
		case resourceManifestName:
			manifestFile = file
		case resourceCacheName:
			cacheFile = file
		case "books":
			if !file.Mode().IsDir() {
				return cache, fmt.Errorf("资源包缺少有效的 books 目录")
			}
		}
	}
	if manifestFile == nil || cacheFile == nil {
		return cache, fmt.Errorf("请选择 wd-reader 导出的资源包")
	}
	var manifest resourceManifest
	if err := readResourceJSON(manifestFile, 4096, &manifest); err != nil {
		return cache, err
	}
	if manifest.Format != "wd-reader-resources" || manifest.Version != 1 {
		return cache, fmt.Errorf("不支持的资源包格式或版本")
	}
	if err := readResourceJSON(cacheFile, resourceCacheLimit, &cache); err != nil {
		return cache, fmt.Errorf("前端缓存数据无效: %w", err)
	}
	root, err := os.OpenRoot(stage)
	if err != nil {
		return cache, err
	}
	defer root.Close()
	for i, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return cache, err
		}
		if !strings.HasPrefix(file.Name, "books/") {
			continue
		}
		name := filepath.FromSlash(strings.TrimSuffix(strings.TrimPrefix(file.Name, "books/"), "/"))
		if name == "." || name == "" {
			continue
		}
		if file.Mode().IsDir() {
			if err := root.MkdirAll(name, 0755); err != nil {
				return cache, err
			}
			continue
		}
		if file.Name == "books/meta-data/metadata.json" && file.UncompressedSize64 > 64<<20 {
			return cache, fmt.Errorf("书籍元数据超过大小限制")
		}
		if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return cache, err
		}
		source, err := file.Open()
		if err != nil {
			return cache, err
		}
		out, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			source.Close()
			return cache, err
		}
		_, copyErr := io.Copy(out, resourceReader{ctx, io.LimitReader(source, int64(file.UncompressedSize64)+1)})
		source.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return cache, copyErr
		}
		if closeErr != nil {
			return cache, closeErr
		}
		if !file.Modified.IsZero() {
			if err := root.Chtimes(name, file.Modified, file.Modified); err != nil {
				return cache, err
			}
		}
		reportResources(report, "validating", i+1, len(archive.File))
	}
	return cache, nil
}
