package epubexport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const HistoryDir = `F:\bk-books`

// History tracks published EPUB files for one source and output directory.
type History struct {
	file   *os.File
	output string
	mu     sync.Mutex
	books  map[string]string
}

type historyEntry struct {
	ID         string    `json:"id"`
	Output     string    `json:"output"`
	File       string    `json:"file"`
	RecordedAt time.Time `json:"recordedAt"`
}

func OpenHistory(path, output string) (*History, error) {
	h := &History{}
	if path == "" {
		return h, nil
	}
	var err error
	h.output, err = filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	h.file, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	h.books = make(map[string]string)
	scanner := bufio.NewScanner(h.file)
	for scanner.Scan() {
		var entry historyEntry
		// A process interrupted during an append may leave a partial line.
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.Output == h.output &&
			entry.ID != "" && filepath.IsLocal(entry.File) && filepath.Ext(entry.File) == ".epub" {
			h.books[entry.ID] = entry.File
		}
	}
	if err := scanner.Err(); err != nil {
		h.file.Close()
		return nil, fmt.Errorf("读取导出记录 %s: %w", path, err)
	}
	return h, nil
}

func (h *History) Close() error {
	if h.file == nil {
		return nil
	}
	return h.file.Close()
}

func (h *History) CompletedPath(id string) string {
	h.mu.Lock()
	name := h.books[id]
	h.mu.Unlock()
	if name == "" {
		return ""
	}
	path := filepath.Join(h.output, name)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return path
	}
	return ""
}

func (h *History) Record(id, path string) error {
	if h.file == nil {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	name, err := filepath.Rel(h.output, abs)
	if err != nil || !filepath.IsLocal(name) || filepath.Ext(name) != ".epub" {
		return fmt.Errorf("无法记录输出目录以外或非 EPUB 的文件: %s", path)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, err := json.Marshal(historyEntry{ID: id, Output: h.output, File: name, RecordedAt: time.Now()})
	if err != nil {
		return err
	}
	// Start on a new line even if a previous append was interrupted.
	if _, err := h.file.Write(append(append([]byte{'\n'}, entry...), '\n')); err != nil {
		return fmt.Errorf("写入导出记录: %w", err)
	}
	if err := h.file.Sync(); err != nil {
		return fmt.Errorf("保存导出记录: %w", err)
	}
	h.books[id] = name
	return nil
}
