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

const defaultOutput = `C:\Users\bokun\Downloads\book-list-jiujiu`

type options struct {
	out, id                            string
	section, startPage, limit, workers int
	list, txtOnly                      bool
}

func main() {
	var opts options
	flag.StringVar(&opts.out, "out", defaultOutput, "EPUB 输出目录")
	flag.StringVar(&opts.id, "book", "", "只处理指定书籍 ID（详情页网址中的数字）")
	flag.IntVar(&opts.section, "section", 0, "列表分区（0 全部，1 女生，2 男生，3 耽美）")
	flag.IntVar(&opts.startPage, "start-page", 1, "从列表第几页开始")
	flag.IntVar(&opts.limit, "limit", 0, "本次最多处理多少本（0 表示全部）")
	flag.IntVar(&opts.workers, "workers", 15, "同时导出的书籍数量")
	flag.BoolVar(&opts.list, "list", false, "只列出书籍，不生成 EPUB")
	flag.BoolVar(&opts.txtOnly, "txt-only", false, "只导出可用整本 TXT 对齐目录的书，跳过需要逐页读取的书")
	delay := flag.Duration("delay", 500*time.Millisecond, "所有书籍共享的请求间隔")
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
	if opts.id != "" {
		b, err := src.details(ctx, opts.id)
		if err != nil {
			return err
		}
		if opts.list {
			fmt.Printf("%s\t%s\t%s\t%s\n", b.id, b.Category, b.Title, b.Author)
			return nil
		}
		return exportBook(ctx, src, opts, b)
	}

	slots := make(chan struct{}, opts.workers)
	var workers sync.WaitGroup
	var failures atomic.Int64
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
				err = exportBook(ctx, src, opts, b)
			}
			if err != nil {
				failures.Add(1)
				log.Printf("书籍 %s《%s》失败: %v", entry.id, entry.title, err)
			}
		})
		return nil
	})
	workers.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if failed := failures.Load(); failed > 0 {
		err = errors.Join(err, fmt.Errorf("处理 %d 本，失败 %d 本", total, failed))
	}
	return err
}

func walkCompleted(ctx context.Context, src *source, opts options, visit func(listing) error) (int, error) {
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
				if seen[entry.id] {
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

func exportBook(ctx context.Context, src *source, opts options, b book) error {
	dir := filepath.Join(opts.out, epubexport.SafeName(b.Category))
	target := filepath.Join(dir, epubexport.SafeName(b.Title)+" ["+b.id+"].epub")
	if info, err := os.Stat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("输出路径已被占用: %s", target)
		}
		log.Printf("跳过已有文件 %s", target)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	log.Printf("导出《%s》 · %s", b.Title, b.Category)
	toc, err := src.catalog(ctx, b.id)
	if err != nil {
		return err
	}
	originalCount := len(toc)
	var bodies [][]string
	text, err := src.downloadTXT(ctx, b)
	if err == nil {
		toc, bodies, err = prepareTXT(text, b.Title, toc)
	}
	completeTXT := err == nil
	missing := 0
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if opts.txtOnly {
			log.Printf("跳过 %s《%s》（需要读取网页正文）: %v", b.id, b.Title, err)
			return nil
		}
		missing = len(toc)
		for _, body := range bodies {
			if len(body) > 0 {
				missing--
			}
		}
		log.Printf("《%s》已复用 %d/%d 章 TXT，需要网页补抓 %d 章: %v", b.Title, len(toc)-missing, len(toc), missing, err)
	} else if len(toc) != originalCount {
		log.Printf("《%s》使用 TXT 原始目录（%d 章）", b.Title, len(toc))
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
	fetched := 0
	err = epubexport.Write(target, b.Book, titles, cover, func(i int) ([]string, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if completeTXT || (i < len(bodies) && len(bodies[i]) > 0) {
			return bodies[i], nil
		}
		if fetched%10 == 0 {
			log.Printf("《%s》正在补抓第 %d/%d 章（目录位置 %d/%d）", b.Title, fetched+1, missing, i+1, len(toc))
		}
		fetched++
		return src.chapterBody(ctx, toc[i])
	})
	if err != nil {
		return err
	}
	log.Printf("已保存 %s（%d 章）", target, len(toc))
	return nil
}
