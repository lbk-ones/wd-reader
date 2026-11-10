package resourcecatalog

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxDownloadBytes 限制单次网络下载及解压后电子书的大小，单位字节。
const MaxDownloadBytes int64 = 100 * 1024 * 1024

// DownloadStatusError 保留成品文件下载失败时的 HTTP 状态和服务器提供的重试提示。
type DownloadStatusError struct {
	// StatusCode 是服务器响应的 HTTP 状态码。
	StatusCode int
	// RetryAfter 是 Retry-After 响应头的原文，可能为空。
	RetryAfter string
}

// Error 生成包含 HTTP 状态码的下载错误提示。
func (e *DownloadStatusError) Error() string {
	return fmt.Sprintf("下载失败 (HTTP %d)", e.StatusCode)
}

// ValidateDownload 校验成品文件的下载地址、扩展名和可选 Referer。ZIP 还须声明内部电子书格式。
// 章节抓取任务需携带匹配的书源配置，由 ValidateCrawlerDownload 校验。
func ValidateDownload(file File) error {
	if file.NovelSourceID != "" {
		return fmt.Errorf("章节下载需要已配置的书源")
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	if !IsWebURL(file.URL) || (!supportedExtension(ext) && !(ext == ".zip" && supportedExtension("."+file.ArchiveFormat))) {
		return fmt.Errorf("仅支持 HTTP(S) 的电子书文件，ZIP 需指定内部电子书格式")
	}
	if file.Referer != "" && !IsWebURL(file.Referer) {
		return fmt.Errorf("下载来源地址无效")
	}
	return nil
}

// Download 将完整资源写入调用方专用临时目录；检查大小和文件头，失败删除半成品，ZIP 返回解压后的文件路径。
// progress 可为空，依次报告已接收字节数和总字节数。
// ctx 控制网络请求及写入循环；返回路径仅在文件下载完成后可用于书架导入。
func Download(ctx context.Context, client *http.Client, file File, dir string, progress func(int64, int64)) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Name))
	if err := ValidateDownload(file); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", catalogUserAgent)
	if file.Referer != "" {
		req.Header.Set("Referer", file.Referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", &DownloadStatusError{StatusCode: resp.StatusCode, RetryAfter: resp.Header.Get("Retry-After")}
	}
	if resp.ContentLength > MaxDownloadBytes {
		return "", fmt.Errorf("文件超过 100 MiB 下载限制")
	}
	// 先窥视文件头，拒绝验证页、空文件和不符合格式特征的数据；Peek 不消耗内容。
	reader := bufio.NewReader(resp.Body)
	prefix, _ := reader.Peek(512)
	if len(prefix) == 0 {
		return "", fmt.Errorf("下载文件为空")
	}
	text := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(string(prefix), "\xef\xbb\xbf")))
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") || strings.HasPrefix(text, "<!doctype html") || strings.HasPrefix(text, "<html") || strings.HasPrefix(text, "<script") {
		return "", fmt.Errorf("链接返回的是网页或验证页面，未下载到电子书")
	}
	if ext == ".epub" && (len(prefix) < 4 || string(prefix[:4]) != "PK\x03\x04") {
		return "", fmt.Errorf("下载内容不是有效的 EPUB 文件")
	}
	if ext == ".zip" && (len(prefix) < 4 || string(prefix[:4]) != "PK\x03\x04") {
		return "", fmt.Errorf("下载内容不是有效的 ZIP 文件")
	}
	if (ext == ".mobi" || ext == ".azw" || ext == ".azw3") && (len(prefix) < 68 || string(prefix[60:68]) != "BOOKMOBI") {
		return "", fmt.Errorf("下载内容不是有效的 MOBI/AZW3 文件")
	}
	name := SafeFileName(file.Name, ext)
	target := filepath.Join(dir, name)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	// 只有完整写入并关闭成功才标记完成；其余返回路径自动删除半成品。
	committed := false
	defer func() {
		out.Close()
		if !committed {
			os.Remove(target)
		}
	}()
	// 即使服务端不提供 Content-Length，也通过实际读取字节数执行大小限制。
	limited := io.LimitReader(reader, MaxDownloadBytes+1)
	buffer := make([]byte, 64*1024)
	var received int64
	last := time.Time{}
	for {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := limited.Read(buffer)
		if n > 0 {
			received += int64(n)
			if received > MaxDownloadBytes {
				return "", fmt.Errorf("文件超过 100 MiB 下载限制")
			}
			if _, err = out.Write(buffer[:n]); err != nil {
				return "", err
			}
			// 节流进度回调，避免频繁获取任务锁；结束时还会补报最后一次进度。
			if progress != nil && time.Since(last) > 150*time.Millisecond {
				progress(received, resp.ContentLength)
				last = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	// 对声明了长度的响应核对实收字节数，避免截断文件进入书架。
	if resp.ContentLength >= 0 && received != resp.ContentLength {
		return "", fmt.Errorf("文件下载不完整，请重试")
	}
	if err = out.Close(); err != nil {
		return "", err
	}
	if progress != nil {
		progress(received, resp.ContentLength)
	}
	committed = true
	if ext == ".zip" {
		return extractBookArchive(ctx, target, file, dir)
	}
	return target, nil
}

// extractBookArchive 从 ZIP 中选择预期格式的最大普通文件，检查内容和解压大小，再以安全文件名写入临时目录。
func extractBookArchive(ctx context.Context, path string, file File, dir string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("无法读取 ZIP: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > 200 {
		return "", fmt.Errorf("ZIP 文件条目过多，请使用单本电子书压缩包")
	}
	var chosen *zip.File
	ext := "." + file.ArchiveFormat
	// 跳过目录和符号链接；同格式有多个文件时取最大的一个作为正文候选。
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || entry.Mode()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name), ext) {
			continue
		}
		if entry.UncompressedSize64 > uint64(MaxDownloadBytes) {
			return "", fmt.Errorf("解压后的电子书超过 100 MiB 限制")
		}
		if chosen == nil || entry.UncompressedSize64 > chosen.UncompressedSize64 {
			chosen = entry
		}
	}
	if chosen == nil {
		return "", fmt.Errorf("ZIP 内未找到 %s 电子书", strings.ToUpper(file.ArchiveFormat))
	}
	in, err := chosen.Open()
	if err != nil {
		return "", err
	}
	defer in.Close()
	input := bufio.NewReader(in)
	prefix, _ := input.Peek(512)
	text := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(string(prefix), "\xef\xbb\xbf")))
	if strings.HasPrefix(text, "<!doctype html") || strings.HasPrefix(text, "<html") || strings.HasPrefix(text, "<script") {
		return "", fmt.Errorf("ZIP 内的文件是网页，未找到电子书正文")
	}
	if ext == ".epub" && (len(prefix) < 4 || string(prefix[:4]) != "PK\x03\x04") {
		return "", fmt.Errorf("ZIP 内不是有效的 EPUB 文件")
	}
	if (ext == ".mobi" || ext == ".azw" || ext == ".azw3") && (len(prefix) < 68 || string(prefix[60:68]) != "BOOKMOBI") {
		return "", fmt.Errorf("ZIP 内不是有效的 MOBI/AZW3 文件")
	}
	// 不使用压缩包内的路径落盘，从下载名称重新生成文件名，避免 ZIP 路径穿越。
	target := filepath.Join(dir, SafeFileName(strings.TrimSuffix(file.Name, filepath.Ext(file.Name)), ext))
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		out.Close()
		if !committed {
			os.Remove(target)
		}
	}()
	buffer := make([]byte, 64*1024)
	// 解压流再次限量，并读到 EOF 以触发 ZIP 校验，不能只信条目声明的大小。
	limited := io.LimitReader(input, MaxDownloadBytes+1)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := limited.Read(buffer)
		if n > 0 {
			total += int64(n)
			if total > MaxDownloadBytes {
				return "", fmt.Errorf("解压后的电子书超过 100 MiB 限制")
			}
			if _, err = out.Write(buffer[:n]); err != nil {
				return "", err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("ZIP 校验失败: %w", readErr)
		}
	}
	if total == 0 {
		return "", fmt.Errorf("ZIP 内的电子书为空")
	}
	if err = out.Close(); err != nil {
		return "", err
	}
	committed = true
	return target, nil
}
