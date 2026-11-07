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

const MaxDownloadBytes int64 = 100 * 1024 * 1024

func ValidateDownload(file File) error {
	ext := strings.ToLower(filepath.Ext(file.Name))
	if !IsWebURL(file.URL) || (!supportedExtension(ext) && !(ext == ".zip" && supportedExtension("."+file.ArchiveFormat))) {
		return fmt.Errorf("仅支持 HTTP(S) 的电子书文件，ZIP 需指定内部电子书格式")
	}
	if file.Referer != "" && !IsWebURL(file.Referer) {
		return fmt.Errorf("下载来源地址无效")
	}
	return nil
}

// Download writes only a complete resource into the caller's dedicated temporary directory.
func Download(ctx context.Context, client *http.Client, file File, dir string, progress func(int64, int64)) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Name))
	if err := ValidateDownload(file); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "wd-reader/1.0 (ebook downloads; https://github.com/lbk-ones/wd-reader)")
	if file.Referer != "" {
		req.Header.Set("Referer", file.Referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("下载失败 (HTTP %d)", resp.StatusCode)
	}
	if resp.ContentLength > MaxDownloadBytes {
		return "", fmt.Errorf("文件超过 100 MiB 下载限制")
	}
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
	if (ext == ".mobi" || ext == ".azw3") && (len(prefix) < 68 || string(prefix[60:68]) != "BOOKMOBI") {
		return "", fmt.Errorf("下载内容不是有效的 MOBI/AZW3 文件")
	}
	name := SafeFileName(file.Name, ext)
	target := filepath.Join(dir, name)
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
	if (ext == ".mobi" || ext == ".azw3") && (len(prefix) < 68 || string(prefix[60:68]) != "BOOKMOBI") {
		return "", fmt.Errorf("ZIP 内不是有效的 MOBI/AZW3 文件")
	}
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
