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

// DownloadTask 是返回给前端的任务快照，前端轮询它更新进度和操作状态。
type DownloadTask struct {
	// 原始下载描述，供失败重试时恢复书源标识、Referer 和 ZIP 格式等参数。
	File resourcecatalog.File `json:"file"`
	// 本次任务的 UUID，用于取消和查询。
	ID string `json:"id"`
	// 清理后的下载文件名。
	Name string `json:"name"`
	// 成品文件或书籍详情地址，与抓取书源标识一起用于识别重复任务。
	URL string `json:"url"`
	// 任务状态：queued、downloading、importing、done、cancelled 或 error。
	Status string `json:"status"`
	// 已接收的字节数，章节书源则为已完成的章节数。
	Received int64 `json:"received"`
	// 总字节数或总章节数。为 0 或负数时表示未知。
	Total int64 `json:"total"`
	// 失败原因，成功或未失败时为空。
	Error string `json:"error,omitempty"`
	// 成功导入书架后返回的书籍名称。
	BookName string `json:"bookName,omitempty"`
	// 任务创建时间，Unix 毫秒，用于排序和清理历史任务。
	CreatedAt int64 `json:"createdAt"`
}

// resourceDownload 将前端可见状态与后台取消函数绑定；状态读写由下载管理器的锁保护。
type resourceDownload struct {
	// 当前任务状态，读取快照或修改字段前均需持有管理器互斥锁。
	task DownloadTask
	// 取消排队或 HTTP 请求，并释放超时上下文资源。
	cancel context.CancelFunc
}

// resourceDownloads 管理当前进程的下载队列和历史；不持久化，也不支持重启后续传。
type resourceDownloads struct {
	// 保护任务表及任务字段的并发访问。
	mu sync.Mutex
	// 按任务 ID 索引的任务表，最多保留约 100 条记录。
	tasks map[string]*resourceDownload
	// 容量为 2 的信号量；排队任务获得槽位后才开始下载，入库结束后释放。
	slots chan struct{}
}

// resourceSourcesPath 返回与应用书库目录一致的独立下载来源配置路径。
func resourceSourcesPath() string {
	return filepath.Join(book.GetAppPath(), "books", "resource-sources.json")
}

// GetResourceSources 向界面返回配置原文和来源；解析失败时附带错误，仍允许用户打开编辑器修复。
func (a *App) GetResourceSources() (resourcecatalog.ConfigFile, error) {
	file, err := resourcecatalog.Load(resourceSourcesPath())
	// 已读到文件但解析失败时保留原文，让编辑器仍可打开并修复配置。
	if err != nil && file.Path != "" {
		file.Error = err.Error()
		file.Sources = []resourcecatalog.Source{}
		return file, nil
	}
	return file, err
}

// SaveResourceSources 校验并保存配置，成功后通知主窗口刷新下载来源。
func (a *App) SaveResourceSources(content string) error {
	if err := resourcecatalog.Save(resourceSourcesPath(), content); err != nil {
		return err
	}
	if a.window != nil {
		a.window.EmitEvent("resource-sources-changed")
	}
	return nil
}

// GetDefaultResourceSources 读取内置默认配置供编辑器恢复草稿，不直接写入磁盘。
func (a *App) GetDefaultResourceSources() string { return resourcecatalog.DefaultConfig() }

// FindDownloadableBooks 每次读取最新配置，仅对存在且已启用的指定来源执行搜索。
func (a *App) FindDownloadableBooks(id, query string) (resourcecatalog.SearchResponse, error) {
	source, err := a.resourceSource(id)
	if err != nil {
		return resourcecatalog.SearchResponse{}, err
	}
	result, err := resourcecatalog.Search(context.Background(), http.DefaultClient, source, query)
	for i := range result.Books {
		for j := range result.Books[i].Files {
			result.Books[i].Files[j].SourceGroup = source.Group
		}
	}
	return result, err
}

// CheckResourceSource 按最新配置探测指定的已启用书源，面板关闭或重新检测时可取消请求。
func (a *App) CheckResourceSource(ctx context.Context, id string) (resourcecatalog.SourceHealth, error) {
	source, err := a.resourceSource(id)
	if err != nil {
		return resourcecatalog.SourceHealth{}, err
	}
	return resourcecatalog.CheckSource(ctx, http.DefaultClient, source), nil
}

// ResolveResourceNovel 从当前配置中选择能处理详情链接的抓取书源，返回书名、作者和下载项。
func (a *App) ResolveResourceNovel(ctx context.Context, link string) (resourcecatalog.Book, error) {
	config, err := resourcecatalog.Load(resourceSourcesPath())
	if err != nil {
		return resourcecatalog.Book{}, err
	}
	return resourcecatalog.ResolveNovelBook(ctx, http.DefaultClient, config.Sources, link)
}

// resourceSource 按标识查找已启用来源。配置无效时优先返回原始配置错误，便于定位原因。
func (a *App) resourceSource(id string) (resourcecatalog.Source, error) {
	config, err := a.GetResourceSources()
	if err != nil {
		return resourcecatalog.Source{}, err
	}
	for _, source := range config.Sources {
		if source.ID == id && source.Enabled {
			return source, nil
		}
	}
	if config.Error != "" {
		return resourcecatalog.Source{}, errors.New(config.Error)
	}
	return resourcecatalog.Source{}, fmt.Errorf("资源来源不存在或已禁用")
}

// ListResourceDownloads 在锁内复制任务快照，按创建时间倒序返回给前端轮询。
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

// StartResourceDownload 校验并去重活动任务，限制队列数量，异步下载和入库。
func (a *App) StartResourceDownload(file resourcecatalog.File) (DownloadTask, error) {
	var source resourcecatalog.Source
	if file.NovelSourceID != "" {
		// 以后端当前配置为准校验来源和地址。任务入队后沿用本次读到的规则快照。
		var err error
		source, err = a.resourceSource(file.NovelSourceID)
		if err != nil {
			return DownloadTask{}, err
		}
		if err := resourcecatalog.ValidateCrawlerDownload(source, file); err != nil {
			return DownloadTask{}, err
		}
		file.SourceGroup = source.Group
	} else if err := resourcecatalog.ValidateDownload(file); err != nil {
		return DownloadTask{}, err
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	if a.downloads.tasks == nil {
		a.downloads.tasks = map[string]*resourceDownload{}
		a.downloads.slots = make(chan struct{}, 2)
	}
	// 地址和抓取书源都相同的活动任务直接复用。终态任务允许重新创建。
	active := 0
	for _, item := range a.downloads.tasks {
		if item.task.Status == "queued" || item.task.Status == "downloading" || item.task.Status == "importing" {
			active++
			if item.task.URL == file.URL && item.task.File.NovelSourceID == file.NovelSourceID {
				return item.task, nil
			}
		}
	}
	// 活动任务包含排队、下载和入库，防止不断提交导致队列无限增长。
	if active >= 20 {
		return DownloadTask{}, fmt.Errorf("下载队列已满，请等待当前任务完成")
	}
	// 历史达到上限时只淘汰最早的终态任务，不能删除协程仍持有的活动任务。
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
	// 超时从创建任务时开始，包含排队时间。逐章任务涉及较多页面，单独使用更长的时限。
	timeout := 5 * time.Minute
	if file.NovelSourceID != "" {
		timeout = 2 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	task := DownloadTask{ID: uuid.NewString(), Name: resourcecatalog.SafeFileName(file.Name, ext), URL: file.URL, Status: "queued", Total: file.Size, CreatedAt: time.Now().UnixMilli()}
	task.File = file
	a.downloads.tasks[task.ID] = &resourceDownload{task: task, cancel: cancel}
	go a.runResourceDownload(ctx, task.ID, file, source)
	return task, nil
}

// CancelResourceDownload 取消排队或下载阶段的任务；进入入库阶段后不再允许取消。
func (a *App) CancelResourceDownload(id string) error {
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	item := a.downloads.tasks[id]
	if item == nil {
		return nil
	}
	// 入库会解析并更新书架文件，此阶段拒绝取消，等待导入流程完成。
	if item.task.Status == "importing" {
		return fmt.Errorf("正在加入书架，请等待完成")
	}
	if item.task.Status == "queued" || item.task.Status == "downloading" {
		item.cancel()
		item.task.Status = "cancelled"
	}
	return nil
}

// stopResourceDownloads 应用退出时取消所有任务的上下文，唤醒排队任务并中断未完成的网络请求。
func (a *App) stopResourceDownloads() {
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	for _, item := range a.downloads.tasks {
		item.cancel()
	}
}

// runResourceDownload 后台执行排队、下载和入库，统一更新终态并清理专用临时目录。
func (a *App) runResourceDownload(ctx context.Context, id string, file resourcecatalog.File, source resourcecatalog.Source) {
	a.downloads.mu.Lock()
	item := a.downloads.tasks[id]
	a.downloads.mu.Unlock()
	defer item.cancel()
	// 统一收敛终态：主动取消为 cancelled，超时为 error，导入成功才记为 done。
	finish := func(name string, err error) {
		a.downloads.mu.Lock()
		defer a.downloads.mu.Unlock()
		if err != nil {
			if ctx.Err() != nil {
				item.task.Status = "cancelled"
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
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
	// 等待并发槽位时也监听取消；获得后直到下载和入库结束才归还。
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
	// 每个任务单独创建临时目录，失败或成功退出时均清理。
	dir, err := os.MkdirTemp("", "wd-reader-resource-")
	if err != nil {
		finish("", err)
		return
	}
	defer func() {
		// 同时检查目录位于系统临时目录内且前缀符合约定，避免误删其他目录。
		if rel, e := filepath.Rel(os.TempDir(), dir); e == nil && !strings.HasPrefix(rel, "..") && strings.HasPrefix(filepath.Base(dir), "wd-reader-resource-") {
			os.RemoveAll(dir)
		}
	}()
	// 下载回调更新传输或章节进度，入库流程单独设置终态。
	progress := func(received, total int64) {
		a.downloads.mu.Lock()
		defer a.downloads.mu.Unlock()
		item.task.Received = received
		item.task.Total = total
	}
	var local string
	// 两类任务分别获取成品，后续均使用同一套入库和取消检查流程。
	if file.NovelSourceID != "" {
		local, err = resourcecatalog.DownloadCrawler(ctx, http.DefaultClient, source, file, dir, progress)
	} else {
		local, err = resourcecatalog.Download(ctx, http.DefaultClient, file, dir, progress)
	}
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
	// 在锁内完成取消检查和入库状态切换，避免已开始导入后又被标记为取消。
	item.task.Status = "importing"
	a.downloads.mu.Unlock()
	// 复用本地导入流程进行解析、保留原文件及同名处理，成功后前端再刷新书架。
	name, err := book.ImportLocal(local)
	finish(name, err)
}
