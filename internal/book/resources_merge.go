package book

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type ResourceImportStats struct {
	FilesAdded   int `json:"filesAdded"`
	FilesSkipped int `json:"filesSkipped"`
	FilesRenamed int `json:"filesRenamed"`
	BooksAdded   int `json:"booksAdded"`
}

type resourceCopy struct{ source, destination string }
type resourceEntry struct {
	name, hash string
	directory  bool
	missing    bool
	planned    bool
}

// ResourceImportPlan stages files without changing the active library.
type ResourceImportPlan struct {
	Cache     FrontendCache
	BookNames map[string]string
	Stats     ResourceImportStats
	stage     string
	books     string
	metadata  metadataStore
	baseline  []byte
	copies    []resourceCopy
	dirs      []string
	checks    map[string]string
}

func optionalResourceFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func resourceHash(ctx context.Context, root *os.Root, name string) (string, error) {
	file, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, resourceReader{ctx, file}); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func importedResourceName(name string, number int) string {
	ext := path.Ext(name)
	return fmt.Sprintf("%s (import %d)%s", strings.TrimSuffix(name, ext), number, ext)
}

func validResourceBookName(name string) bool {
	return validResourcePath(name) && !strings.Contains(name, "/")
}

func decodeResourceMetadata(data []byte) (metadataStore, error) {
	store := metadataStore{Version: 1, Books: []BookMetadata{}}
	if data == nil {
		return store, nil
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, err
	}
	if store.Version != 1 {
		return store, fmt.Errorf("不支持的书籍信息版本")
	}
	categories := map[string]bool{"": true}
	for _, category := range store.Categories {
		if category.ID == "" || categories[category.ID] || strings.TrimSpace(category.Name) == "" {
			return store, fmt.Errorf("分类信息无效")
		}
		categories[category.ID] = true
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, book := range store.Books {
		if book.ID == "" || ids[book.ID] || !validResourceBookName(book.FileName) || names[strings.ToLower(book.FileName)] ||
			(book.OriginalName != "" && !validResourceBookName(book.OriginalName)) ||
			(book.Cover != "" && !validResourceBookName(book.Cover)) || !categories[book.CategoryID] {
			return store, fmt.Errorf("书籍信息或关联路径无效")
		}
		ids[book.ID], names[strings.ToLower(book.FileName)] = true, true
	}
	return store, nil
}

// PrepareResourceImport extracts, validates and plans collisions before any live writes.
func PrepareResourceImport(ctx context.Context, filename string, report func(ResourceProgress)) (_ *ResourceImportPlan, resultErr error) {
	stage, err := os.MkdirTemp("", "wd-reader-resources-*")
	if err != nil {
		return nil, err
	}
	p := &ResourceImportPlan{stage: stage, BookNames: map[string]string{}, checks: map[string]string{}}
	defer func() {
		if resultErr != nil {
			p.Close()
		}
	}()
	p.Cache, err = extractResourceArchive(ctx, filename, stage, report)
	if err != nil {
		return nil, err
	}
	data, err := optionalResourceFile(filepath.Join(stage, "meta-data", "metadata.json"))
	if err != nil {
		return nil, err
	}
	incoming, err := decodeResourceMetadata(data)
	if err != nil {
		return nil, fmt.Errorf("资源包书籍信息无效: %w", err)
	}
	importMu.Lock()
	defer importMu.Unlock()
	if err := EnsureStorage(); err != nil {
		return nil, err
	}
	storageMu.Lock()
	defer storageMu.Unlock()
	metadataMu.Lock()
	defer metadataMu.Unlock()
	p.books, err = filepath.Abs(resourcesPath())
	if err != nil {
		return nil, err
	}
	p.metadata, err = readMetadata()
	if err != nil {
		return nil, err
	}
	p.baseline, err = optionalResourceFile(filepath.Join(metadataDir(), "metadata.json"))
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(p.books)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	source, err := os.OpenRoot(stage)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	currentEntries, err := resourceEntries(p.books)
	if err != nil {
		return nil, err
	}
	index := map[string]resourceEntry{}
	for _, name := range currentEntries {
		info, err := root.Stat(filepath.FromSlash(name))
		if err != nil {
			return nil, err
		}
		index[strings.ToLower(name)] = resourceEntry{name: name, directory: info.IsDir()}
	}
	for _, book := range p.metadata.Books {
		name := "parse/" + book.FileName
		if _, exists := index[strings.ToLower(name)]; !exists {
			index[strings.ToLower(name)] = resourceEntry{name: name, hash: book.FileHash, missing: true}
		}
	}
	entries, err := resourceEntries(stage)
	if err != nil {
		return nil, err
	}
	paths := map[string]string{}
	incomingNames := map[string]bool{}
	for _, name := range entries {
		incomingNames[strings.ToLower(name)] = true
	}
	for _, name := range entries {
		info, err := source.Stat(filepath.FromSlash(name))
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			p.dirs = append(p.dirs, name)
			continue
		}
		if name == "meta-data/metadata.json" || name == "meta-data/metadata.json.bak" {
			continue // The active metadata and its rollback copy are managed together.
		}
		hash, err := resourceHash(ctx, source, name)
		if err != nil {
			return nil, err
		}
		destination := name
		planned := false
		for number := 2; ; number++ {
			if destination != name && incomingNames[strings.ToLower(destination)] {
				destination = importedResourceName(name, number)
				continue
			}
			entry, exists := index[strings.ToLower(destination)]
			if !exists {
				p.copies = append(p.copies, resourceCopy{name, destination})
				planned = true
				break
			}
			if !entry.directory {
				if entry.hash == "" && !entry.missing {
					entry.hash, err = resourceHash(ctx, root, entry.name)
					if err != nil {
						return nil, err
					}
				}
				if hash == entry.hash {
					destination = entry.name
					if entry.missing {
						p.copies = append(p.copies, resourceCopy{name, destination})
						planned = true
					} else {
						if !entry.planned {
							p.checks[destination] = hash
						}
						p.Stats.FilesSkipped++
					}
					break
				}
			}
			destination = importedResourceName(name, number)
		}
		index[strings.ToLower(destination)] = resourceEntry{name: destination, hash: hash, planned: planned || index[strings.ToLower(destination)].planned}
		paths[name] = destination
		if !strings.EqualFold(name, destination) {
			p.Stats.FilesRenamed++
		}
		if path.Dir(name) == "parse" {
			p.BookNames[path.Base(name)] = path.Base(destination)
		}
	}
	p.Stats.FilesAdded = len(p.copies)
	if err := p.mergeMetadata(incoming, paths, index); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *ResourceImportPlan) mergeMetadata(incoming metadataStore, paths map[string]string, index map[string]resourceEntry) error {
	categoryIDs := map[string]string{}
	usedCategoryIDs := map[string]bool{}
	for _, category := range p.metadata.Categories {
		usedCategoryIDs[category.ID] = true
	}
	for _, category := range incoming.Categories {
		id := ""
		for _, current := range p.metadata.Categories {
			if strings.EqualFold(current.Name, category.Name) {
				id = current.ID
				break
			}
		}
		if id == "" {
			id = category.ID
			if usedCategoryIDs[id] {
				id = uuid.NewString()
			}
			p.metadata.Categories = append(p.metadata.Categories, Category{ID: id, Name: category.Name})
			usedCategoryIDs[id] = true
		}
		categoryIDs[category.ID] = id
	}
	usedIDs := map[string]bool{}
	byName := map[string]bool{}
	for _, book := range p.metadata.Books {
		usedIDs[book.ID] = true
		byName[strings.ToLower(book.FileName)] = true
	}
	for _, book := range incoming.Books {
		oldName := book.FileName
		if mapped, ok := p.BookNames[oldName]; ok {
			book.FileName = mapped
		} else {
			// A missing-file record must not accidentally attach to another local book.
			matched := false
			for _, current := range p.metadata.Books {
				if current.ID == book.ID && current.FileHash == book.FileHash &&
					(current.FileName == oldName || book.FileHash != "") {
					book.FileName = current.FileName
					matched = true
					break
				}
			}
			if !matched {
				for number := 2; ; number++ {
					if _, exists := index[strings.ToLower("parse/"+book.FileName)]; !exists {
						break
					}
					book.FileName = importedResourceName(oldName, number)
				}
			}
			p.BookNames[oldName] = book.FileName
			index[strings.ToLower("parse/"+book.FileName)] = resourceEntry{name: "parse/" + book.FileName, missing: true}
		}
		if byName[strings.ToLower(book.FileName)] {
			continue
		}
		if mapped, ok := paths["pure/"+book.OriginalName]; ok {
			book.OriginalName = path.Base(mapped)
		}
		if mapped, ok := paths["meta-data/covers/"+book.Cover]; ok {
			book.Cover = path.Base(mapped)
		}
		book.CategoryID = categoryIDs[book.CategoryID]
		if usedIDs[book.ID] {
			book.ID = uuid.NewString()
		}
		usedIDs[book.ID] = true
		byName[strings.ToLower(book.FileName)] = true
		book.FileModTime = 0 // Reconcile fingerprints with the imported filesystem.
		p.metadata.Books = append(p.metadata.Books, book)
		p.Stats.BooksAdded++
	}
	p.metadata.CategoryPresetsAdded = p.metadata.CategoryPresetsAdded || incoming.CategoryPresetsAdded
	return nil
}

func ensureResourceDirectory(root *os.Root, name string, created *[]string) error {
	if name == "." || name == "" {
		return nil
	}
	info, err := root.Lstat(filepath.FromSlash(name))
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("资源目标不是普通目录: %s", name)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := ensureResourceDirectory(root, path.Dir(name), created); err != nil {
		return err
	}
	if err := root.Mkdir(filepath.FromSlash(name), 0755); err != nil {
		return err
	}
	*created = append(*created, name)
	return nil
}

// Commit rechecks the plan and only adds files; existing metadata is replaced last.
func (p *ResourceImportPlan) Commit(ctx context.Context, report func(ResourceProgress)) (resultErr error) {
	importMu.Lock()
	defer importMu.Unlock()
	storageMu.Lock()
	defer storageMu.Unlock()
	metadataMu.Lock()
	defer metadataMu.Unlock()
	if current, err := filepath.Abs(resourcesPath()); err != nil || current != p.books {
		return fmt.Errorf("书籍目录已变化，请重新导入")
	}
	current, err := optionalResourceFile(filepath.Join(p.books, "meta-data", "metadata.json"))
	if err != nil {
		return err
	}
	if !bytes.Equal(current, p.baseline) {
		return fmt.Errorf("书库已变化，请重新选择资源包导入")
	}
	root, err := os.OpenRoot(p.books)
	if err != nil {
		return err
	}
	defer root.Close()
	source, err := os.OpenRoot(p.stage)
	if err != nil {
		return err
	}
	defer source.Close()
	for name, expected := range p.checks {
		actual, err := resourceHash(ctx, root, name)
		if err != nil || expected != actual {
			return fmt.Errorf("书库文件已变化，请重新导入: %s", name)
		}
	}
	var createdFiles, createdDirs []string
	defer func() {
		if resultErr != nil {
			for i := len(createdFiles) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, root.Remove(filepath.FromSlash(createdFiles[i])))
			}
			for i := len(createdDirs) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, root.Remove(filepath.FromSlash(createdDirs[i])))
			}
		}
	}()
	for _, name := range p.dirs {
		if err := ensureResourceDirectory(root, name, &createdDirs); err != nil {
			return err
		}
	}
	for i, file := range p.copies {
		if err := ensureResourceDirectory(root, path.Dir(file.destination), &createdDirs); err != nil {
			return err
		}
		in, err := source.Open(filepath.FromSlash(file.source))
		if err != nil {
			return err
		}
		out, err := root.OpenFile(filepath.FromSlash(file.destination), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			in.Close()
			return err
		}
		createdFiles = append(createdFiles, file.destination)
		_, copyErr := io.Copy(out, resourceReader{ctx, in})
		info, statErr := in.Stat()
		in.Close()
		if copyErr == nil {
			copyErr = out.Sync()
		}
		closeErr := out.Close()
		if err := errors.Join(copyErr, closeErr, statErr); err != nil {
			return err
		}
		if err := root.Chtimes(filepath.FromSlash(file.destination), info.ModTime(), info.ModTime()); err != nil {
			return err
		}
		reportResources(report, "importing", i+1, len(p.copies))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backupPath := filepath.Join(p.books, "meta-data", "metadata.json.bak")
	backup, err := optionalResourceFile(backupPath)
	if err != nil {
		return err
	}
	if err := writeMetadata(p.metadata); err != nil {
		if backup != nil {
			err = errors.Join(err, os.WriteFile(backupPath, backup, 0644))
		} else if removeErr := os.Remove(backupPath); !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
		return err
	}
	return nil
}

func (p *ResourceImportPlan) Close() error { return os.RemoveAll(p.stage) }
