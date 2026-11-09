package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"wd-reader/internal/epubexport"
)

const defaultOutput = `F:\bk-books\book-list-jiujiu`

type options struct {
	historyPath                        string
	out, id                            string
	section, startPage, limit, workers int
	list, txtOnly                      bool
}

func main() {
	opts := options{historyPath: filepath.Join(epubexport.HistoryDir, "jjxs-export.jsonl")}
	flag.StringVar(&opts.out, "out", defaultOutput, "EPUB 输出目录")
	flag.StringVar(&opts.id, "book", "", "只处理指定书籍 ID（详情页网址中的数字）")
	flag.IntVar(&opts.section, "section", 0, "列表分区（0 全部，1 女生，2 男生，3 耽美）")
	flag.IntVar(&opts.startPage, "start-page", 1, "从列表第几页开始")
	flag.IntVar(&opts.limit, "limit", 0, "本次最多处理多少本（0 表示全部）")
	flag.IntVar(&opts.workers, "workers", 15, "同时导出的书籍数量")
	flag.BoolVar(&opts.list, "list", false, "只列出书籍，不生成 EPUB")
	flag.BoolVar(&opts.txtOnly, "txt-only", true, "只使用整本下载；设为 false 时允许下载失败后逐页补抓")
	delay := flag.Duration("delay", 1000*time.Millisecond, "所有书籍共享的请求间隔")
	flag.Parse()
	if flag.NArg() != 0 || opts.section < 0 || opts.section > 3 || opts.startPage < 1 ||
		opts.limit < 0 || opts.workers < 1 || *delay < 0 ||
		(opts.id != "" && !bookPath.MatchString("/txt/"+opts.id+".html")) {
		flag.Usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, newSource(*delay), opts); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, src *source, opts options) error {
	if opts.list {
		opts.historyPath = ""
	}
	history, err := epubexport.OpenHistory(opts.historyPath, opts.out)
	if err != nil {
		return fmt.Errorf("打开导出记录: %w", err)
	}
	defer history.Close()
	if opts.historyPath != "" && opts.id == "" {
		log.Printf("使用导出记录 %s", opts.historyPath)
	}

	if opts.id != "" {
		b, err := src.details(ctx, opts.id)
		if err != nil {
			return err
		}
		if opts.list {
			fmt.Printf("%s\t%s\t%s\t%s\n", b.id, b.Category, b.Title, b.Author)
			return nil
		}
		return exportBook(ctx, src, opts, b, history)
	}

	slots := make(chan struct{}, opts.workers)
	var workers sync.WaitGroup
	var failures atomic.Int64
	var deferred atomic.Int64
	total, err := walkCompleted(ctx, src, opts, func(entry listing) error {
		if opts.list {
			fmt.Printf("%s\t%s\n", entry.id, entry.title)
			return nil
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		workers.Go(func() {
			defer func() { <-slots }()
			b, err := src.details(ctx, entry.id)
			if err == nil {
				err = exportBook(ctx, src, opts, b, history)
			}
			if errors.Is(err, errRateLimited) {
				deferred.Add(1)
				log.Printf("书籍 %s《%s》因限流跳过，可下次重跑: %v", entry.id, entry.title, err)
			} else if err != nil {
				failures.Add(1)
				log.Printf("书籍 %s《%s》失败: %v", entry.id, entry.title, err)
			}
		})
		return nil
	}, func(id string) bool {
		return history.CompletedPath(id) != ""
	})
	workers.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if failed := failures.Load(); failed > 0 {
		err = errors.Join(err, fmt.Errorf("处理 %d 本，失败 %d 本", total, failed))
	}
	if skipped := deferred.Load(); skipped > 0 {
		err = errors.Join(err, fmt.Errorf("因限流跳过 %d 本，可下次重跑", skipped))
	}
	return err
}

func walkCompleted(ctx context.Context, src *source, opts options, visit func(listing) error, skip func(string) bool) (int, error) {
	sections := []int{1, 2, 3}
	if opts.section != 0 {
		sections = []int{opts.section}
	}
	seen := make(map[string]bool)
	// 轮流访问各分区，限量导出时也能覆盖不同分区。
	for page := opts.startPage; len(sections) > 0; page++ {
		active := sections[:0]
		for _, section := range sections {
			entries, next, err := src.completed(ctx, section, page)
			if err != nil {
				return len(seen), err
			}
			for _, entry := range entries {
				if seen[entry.id] || (skip != nil && skip(entry.id)) {
					continue
				}
				if err := visit(entry); err != nil {
					return len(seen), err
				}
				seen[entry.id] = true
				if opts.limit > 0 && len(seen) >= opts.limit {
					return len(seen), nil
				}
			}
			if next {
				active = append(active, section)
			}
		}
		sections = active
	}
	return len(seen), ctx.Err()
}

func exportBook(ctx context.Context, src *source, opts options, b book, history *epubexport.History) error {
	dir := filepath.Join(opts.out, epubexport.SafeName(b.Category))
	target := filepath.Join(dir, epubexport.SafeName(b.Title)+" ["+b.id+"].epub")
	if info, err := os.Stat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("输出路径已被占用: %s", target)
		}
		//log.Printf("跳过已有文件 %s", target)
		return history.Record(b.id, target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	//log.Printf("导出《%s》 · %s", b.Title, b.Category)
	text, downloadErr := src.downloadTXT(ctx, b)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(downloadErr, errRateLimited) {
		return downloadErr
	}
	if downloadErr != nil && opts.txtOnly {
		log.Printf("跳过 %s《%s》（无可用的整本下载）: %v", b.id, b.Title, downloadErr)
		return nil
	}

	toc, catalogErr := src.catalog(ctx, b.id)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(catalogErr, errRateLimited) {
		return catalogErr
	}
	var bodies [][]string
	if downloadErr == nil {
		originalCount := len(toc)
		var splitErr error
		toc, bodies, splitErr = prepareTXT(text, b.Title, toc)
		if splitErr != nil {
			var err error
			toc, bodies, err = wholeTXT(text, b.Title)
			if err != nil {
				return err
			}
			log.Printf("《%s》TXT 与网页目录无法对齐，按整本 TXT 重新分章（%d 章）: %v", b.Title, len(toc), splitErr)
		} else if catalogErr != nil || len(toc) != originalCount {
			log.Printf("《%s》按 TXT 正文结构导出（%d 章）", b.Title, len(toc))
		}
	} else {
		if catalogErr != nil {
			return errors.Join(downloadErr, catalogErr)
		}
		log.Printf("《%s》整本下载失败，改用网页正文（%d 章）: %v", b.Title, len(toc), downloadErr)
	}
	cover, err := src.coverImage(ctx, b.cover)
	if err != nil {
		return fmt.Errorf("获取封面: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	titles := make([]string, len(toc))
	for i, ch := range toc {
		titles[i] = ch.title
	}
	err = epubexport.Write(target, b.Book, titles, cover, func(i int) ([]string, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if bodies != nil {
			return bodies[i], nil
		}
		if i%10 == 0 {
			log.Printf("《%s》正在补抓第 %d/%d 章", b.Title, i+1, len(toc))
		}
		return src.chapterBody(ctx, toc[i])
	})
	if err != nil {
		return err
	}
	log.Printf("已保存 %s（%d 章）", target, len(toc))
	return history.Record(b.id, target)
}
