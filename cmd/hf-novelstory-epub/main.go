package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wd-reader/internal/epubexport"
	"wd-reader/internal/resourcecatalog"
)

const defaultOutput = `F:\bk-books\book-list-hf-novelstory`

type options struct {
	out, book      string
	limit, workers int
	list           bool
}

func main() {
	var opts options
	flag.StringVar(&opts.out, "out", defaultOutput, "EPUB 输出目录")
	flag.StringVar(&opts.book, "book", "", "指定完整书名或数据集中的 TXT 相对路径")
	flag.IntVar(&opts.limit, "limit", 0, "本次最多处理多少本（0 表示全部）")
	flag.IntVar(&opts.workers, "workers", 10, "同时导出的书籍数量")
	flag.BoolVar(&opts.list, "list", false, "仅列出 TXT 文件，不下载")
	flag.Parse()
	if flag.NArg() != 0 || opts.limit < 0 || opts.workers < 1 || strings.TrimSpace(opts.out) == "" {
		flag.Usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, &http.Client{Timeout: 2 * time.Minute}, opts); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, client *http.Client, opts options) error {
	config, err := resourcecatalog.Parse([]byte(resourcecatalog.DefaultConfig()))
	if err != nil {
		return err
	}
	for _, source := range config.Sources {
		if source.ID == "hf-novelstory" {
			books, err := resourcecatalog.ListIndexedBooks(ctx, client, source)
			if err != nil {
				return err
			}
			return exportSelection(ctx, client, opts, books)
		}
	}
	return fmt.Errorf("缺少 hf-novelstory 来源配置")
}

func exportSelection(ctx context.Context, client *http.Client, opts options, books []resourcecatalog.Book) error {
	var selection []resourcecatalog.Book
	seen := make(map[string]bool)
	for _, b := range books {
		if !strings.EqualFold(path.Ext(b.ID), ".txt") || seen[b.ID] {
			continue
		}
		seen[b.ID] = true
		if opts.book == "" || b.ID == opts.book || b.Title == opts.book {
			selection = append(selection, b)
		}
	}
	if len(selection) == 0 {
		return fmt.Errorf("未找到符合条件的 TXT 文件")
	}
	if opts.book != "" && len(selection) > 1 {
		return fmt.Errorf("有多个同名文件，请用 -list 查看路径并用 -book 指定其中一个")
	}
	if opts.limit > 0 {
		selection = selection[:min(opts.limit, len(selection))]
	}
	if opts.list {
		for _, b := range selection {
			fmt.Printf("%s\t%d 字节\n", b.ID, b.Files[0].Size)
		}
		return nil
	}

	cover, err := epubexport.DefaultCover()
	if err != nil {
		return err
	}
	slots := make(chan struct{}, min(opts.workers, len(selection)))
	var workers sync.WaitGroup
	var failures atomic.Int64
enqueue:
	for _, b := range selection {
		select {
		case <-ctx.Done():
			break enqueue
		case slots <- struct{}{}:
		}
		workers.Go(func() {
			defer func() { <-slots }()
			if err := exportBook(ctx, client, opts.out, b, cover); err != nil {
				failures.Add(1)
				log.Printf("《%s》导出失败: %v", b.Title, err)
			}
		})
	}
	workers.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if failed := failures.Load(); failed > 0 {
		return fmt.Errorf("处理 %d 本，失败 %d 本", len(selection), failed)
	}
	return nil
}

func exportBook(ctx context.Context, client *http.Client, out string, b resourcecatalog.Book, cover []byte) error {
	temp, err := os.MkdirTemp("", "hf-novelstory-epub-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	file, err := resourcecatalog.Download(ctx, client, b.Files[0], temp, nil)
	if err != nil {
		return err
	}
	defer os.Remove(file)
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	meta, titles, bodies, err := parseBook(data, b)
	if err != nil {
		return err
	}
	dir := filepath.Join(out, epubexport.SafeName(meta.Category))
	id := sha256.Sum256([]byte(b.ID))
	target := filepath.Join(dir, fmt.Sprintf("%s [%x].epub", epubexport.SafeName(meta.Title), id[:6]))
	if info, err := os.Stat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("输出路径已被占用: %s", target)
		}
		log.Printf("跳过已有文件 %s", target)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	err = epubexport.Write(target, meta, titles, cover, func(i int) ([]string, error) {
		return bodies[i], ctx.Err()
	})
	if err != nil {
		return err
	}
	log.Printf("已保存 %s（%d 章）", target, len(titles))
	return nil
}
