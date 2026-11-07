package desktop

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"wd-reader/internal/book"
	"wd-reader/internal/resourcecatalog"

	"github.com/google/uuid"
)

type DownloadTask struct {
	File      resourcecatalog.File `json:"file"`
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	URL       string               `json:"url"`
	Status    string               `json:"status"`
	Received  int64                `json:"received"`
	Total     int64                `json:"total"`
	Error     string               `json:"error,omitempty"`
	BookName  string               `json:"bookName,omitempty"`
	CreatedAt int64                `json:"createdAt"`
}
type resourceDownload struct {
	task   DownloadTask
	cancel context.CancelFunc
}
type resourceDownloads struct {
	mu    sync.Mutex
	tasks map[string]*resourceDownload
	slots chan struct{}
}

func resourceSourcesPath() string {
	return filepath.Join(book.GetAppPath(), "books", "resource-sources.json")
}
func (a *App) GetResourceSources() (resourcecatalog.ConfigFile, error) {
	file, err := resourcecatalog.Load(resourceSourcesPath())
	if err != nil && file.Path != "" {
		file.Error = err.Error()
		file.Sources = []resourcecatalog.Source{}
		return file, nil
	}
	return file, err
}
func (a *App) SaveResourceSources(content string) error {
	return resourcecatalog.Save(resourceSourcesPath(), content)
}
func (a *App) FindDownloadableBooks(id, query string) (resourcecatalog.SearchResponse, error) {
	config, err := resourcecatalog.Load(resourceSourcesPath())
	if err != nil {
		return resourcecatalog.SearchResponse{}, err
	}
	for _, source := range config.Sources {
		if source.ID == id && source.Enabled {
			return resourcecatalog.Search(context.Background(), http.DefaultClient, source, query)
		}
	}
	return resourcecatalog.SearchResponse{}, fmt.Errorf("资源来源不存在或已禁用")
}
func (a *App) ListResourceDownloads() []DownloadTask {
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	tasks := []DownloadTask{}
	for _, item := range a.downloads.tasks {
		tasks = append(tasks, item.task)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt > tasks[j].CreatedAt })
	return tasks
}
func (a *App) StartResourceDownload(file resourcecatalog.File) (DownloadTask, error) {
	if err := resourcecatalog.ValidateDownload(file); err != nil {
		return DownloadTask{}, err
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	if a.downloads.tasks == nil {
		a.downloads.tasks = map[string]*resourceDownload{}
		a.downloads.slots = make(chan struct{}, 2)
	}
	active := 0
	for _, item := range a.downloads.tasks {
		if item.task.Status == "queued" || item.task.Status == "downloading" || item.task.Status == "importing" {
			active++
			if item.task.URL == file.URL {
				return item.task, nil
			}
		}
	}
	if active >= 20 {
		return DownloadTask{}, fmt.Errorf("下载队列已满，请等待当前任务完成")
	}
	if len(a.downloads.tasks) >= 100 {
		var oldestID string
		oldest := int64(1<<63 - 1)
		for id, item := range a.downloads.tasks {
			if item.task.Status != "queued" && item.task.Status != "downloading" && item.task.Status != "importing" && item.task.CreatedAt < oldest {
				oldest = item.task.CreatedAt
				oldestID = id
			}
		}
		if oldestID != "" {
			delete(a.downloads.tasks, oldestID)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	task := DownloadTask{ID: uuid.NewString(), Name: resourcecatalog.SafeFileName(file.Name, ext), URL: file.URL, Status: "queued", Total: file.Size, CreatedAt: time.Now().UnixMilli()}
	task.File = file
	a.downloads.tasks[task.ID] = &resourceDownload{task: task, cancel: cancel}
	go a.runResourceDownload(ctx, task.ID, file)
	return task, nil
}
func (a *App) CancelResourceDownload(id string) error {
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	item := a.downloads.tasks[id]
	if item == nil {
		return nil
	}
	if item.task.Status == "importing" {
		return fmt.Errorf("正在加入书架，请等待完成")
	}
	if item.task.Status == "queued" || item.task.Status == "downloading" {
		item.cancel()
		item.task.Status = "cancelled"
	}
	return nil
}
func (a *App) stopResourceDownloads() {
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	for _, item := range a.downloads.tasks {
		item.cancel()
	}
}
func (a *App) runResourceDownload(ctx context.Context, id string, file resourcecatalog.File) {
	a.downloads.mu.Lock()
	item := a.downloads.tasks[id]
	a.downloads.mu.Unlock()
	defer item.cancel()
	finish := func(name string, err error) {
		a.downloads.mu.Lock()
		defer a.downloads.mu.Unlock()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				item.task.Status = "cancelled"
				if errors.Is(err, context.DeadlineExceeded) {
					item.task.Status = "error"
					item.task.Error = "下载超时，请重试"
				}
			} else {
				item.task.Status = "error"
				item.task.Error = err.Error()
			}
		} else {
			item.task.Status = "done"
			item.task.BookName = name
		}
	}
	select {
	case a.downloads.slots <- struct{}{}:
		defer func() { <-a.downloads.slots }()
	case <-ctx.Done():
		finish("", ctx.Err())
		return
	}
	a.downloads.mu.Lock()
	if ctx.Err() != nil {
		a.downloads.mu.Unlock()
		finish("", ctx.Err())
		return
	}
	item.task.Status = "downloading"
	a.downloads.mu.Unlock()
	dir, err := os.MkdirTemp("", "wd-reader-resource-")
	if err != nil {
		finish("", err)
		return
	}
	defer func() {
		if rel, e := filepath.Rel(os.TempDir(), dir); e == nil && !strings.HasPrefix(rel, "..") && strings.HasPrefix(filepath.Base(dir), "wd-reader-resource-") {
			os.RemoveAll(dir)
		}
	}()
	local, err := resourcecatalog.Download(ctx, http.DefaultClient, file, dir, func(received, total int64) {
		a.downloads.mu.Lock()
		defer a.downloads.mu.Unlock()
		item.task.Received = received
		item.task.Total = total
	})
	if err != nil {
		finish("", err)
		return
	}
	a.downloads.mu.Lock()
	if ctx.Err() != nil {
		a.downloads.mu.Unlock()
		finish("", ctx.Err())
		return
	}
	item.task.Status = "importing"
	a.downloads.mu.Unlock()
	name, err := book.ImportLocal(local)
	finish(name, err)
}
