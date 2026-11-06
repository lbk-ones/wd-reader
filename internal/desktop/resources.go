package desktop

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"wd-reader/internal/book"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type resourceTransfers struct {
	sync.Mutex
	token string
	plan  *book.ResourceImportPlan
}

type ResourceExportResult struct {
	Path  string `json:"path"`
	Files int    `json:"files"`
}

type ResourceImportPreview struct {
	Token     string                   `json:"token"`
	Cache     book.FrontendCache       `json:"cache"`
	BookNames map[string]string        `json:"bookNames"`
	Stats     book.ResourceImportStats `json:"stats"`
}

func (a *App) resourceProgress(progress book.ResourceProgress) {
	a.application.Event.Emit("reader:resource-progress", progress)
}

func cancelledResourceDialog(err error) bool {
	return err != nil && err.Error() == "cancelled by user"
}

func (a *App) ExportResources(cache book.FrontendCache) (*ResourceExportResult, error) {
	if a.application == nil || !application.System.IsDesktop() {
		return nil, fmt.Errorf("资源导入导出仅支持桌面版本")
	}
	if !a.resources.TryLock() {
		return nil, fmt.Errorf("资源操作正在进行，请稍候")
	}
	defer a.resources.Unlock()
	filename, err := a.application.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title: "导出资源 / Export resources", Filename: "wd-reader-resources-" + time.Now().Format("20060102-150405") + ".zip",
		Filters:              []application.FileFilter{{DisplayName: "ZIP (*.zip)", Pattern: "*.zip"}},
		CanCreateDirectories: true, Window: a.window,
	}).PromptForSingleSelection()
	if cancelledResourceDialog(err) || (err == nil && filename == "") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
		filename += ".zip"
	}
	count, err := book.ExportResources(a.application.Context(), filename, cache, a.resourceProgress)
	if err != nil {
		return nil, err
	}
	return &ResourceExportResult{Path: filename, Files: count}, nil
}

func (a *App) PrepareResourceImport() (*ResourceImportPreview, error) {
	if a.application == nil || !application.System.IsDesktop() {
		return nil, fmt.Errorf("资源导入导出仅支持桌面版本")
	}
	if !a.resources.TryLock() {
		return nil, fmt.Errorf("资源操作正在进行，请稍候")
	}
	defer a.resources.Unlock()
	a.discardResourcePlan()
	filename, err := a.application.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "导入资源 / Import resources", CanChooseFiles: true, Window: a.window,
		Filters: []application.FileFilter{{DisplayName: "ZIP (*.zip)", Pattern: "*.zip"}},
	}).PromptForSingleSelection()
	if cancelledResourceDialog(err) || (err == nil && filename == "") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plan, err := book.PrepareResourceImport(a.application.Context(), filename, a.resourceProgress)
	if err != nil {
		return nil, err
	}
	a.resources.plan, a.resources.token = plan, uuid.NewString()
	return &ResourceImportPreview{Token: a.resources.token, Cache: plan.Cache, BookNames: plan.BookNames, Stats: plan.Stats}, nil
}

func (a *App) CommitResourceImport(token string) (*book.ResourceImportStats, error) {
	a.resources.Lock()
	defer a.resources.Unlock()
	if a.resources.plan == nil || token != a.resources.token {
		return nil, fmt.Errorf("资源导入已失效，请重新选择资源包")
	}
	defer a.discardResourcePlan()
	if err := a.resources.plan.Commit(a.application.Context(), a.resourceProgress); err != nil {
		return nil, err
	}
	stats := a.resources.plan.Stats
	return &stats, nil
}

func (a *App) discardResourcePlan() {
	if a.resources.plan != nil {
		_ = a.resources.plan.Close()
	}
	a.resources.plan, a.resources.token = nil, ""
}

func (a *App) DiscardResourceImport(token string) {
	a.resources.Lock()
	defer a.resources.Unlock()
	if token == "" || token == a.resources.token {
		a.discardResourcePlan()
	}
}
