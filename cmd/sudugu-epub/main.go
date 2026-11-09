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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wd-reader/internal/epubexport"
)

const defaultOutput = `F:\bk-books\book-list`

// 速读谷有些章节把引号以转义文本写进正文。
var chapterText = strings.NewReplacer(`\"`, `"`, `\u0027`, "'")

type options struct {
	historyPath string
	out         string
	bookID      string
	startPage   int
	limit       int
	workers     int
	list        bool
	delay       time.Duration
}

func main() {
	opts := options{historyPath: filepath.Join(epubexport.HistoryDir, "sudugu-export.jsonl")}
	flag.StringVar(&opts.out, "out", defaultOutput, "EPUB 输出目录")
	flag.StringVar(&opts.bookID, "book", "", "只处理指定书籍 ID（详情页网址中的数字）")
	flag.IntVar(&opts.startPage, "start-page", 1, "从完本榜第几页开始")
	flag.IntVar(&opts.limit, "limit", 0, "本次最多处理多少本（0 表示全部）")
	flag.IntVar(&opts.workers, "workers", 10, "同时导出的书籍数量（1 表示逐本导出）")
	flag.BoolVar(&opts.list, "list", false, "只列出书籍，不生成 EPUB")
	flag.DurationVar(&opts.delay, "delay", 100*time.Millisecond, "所有书籍共享的请求间隔")
	flag.Parse()
	if flag.NArg() != 0 || opts.startPage < 1 || opts.limit < 0 || opts.workers < 1 || opts.delay < 0 ||
		(opts.bookID != "" && !bookPath.MatchString("/"+opts.bookID+"/")) {
		flag.Usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, newSource(), opts); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, src *source, opts options) error {
	src.delay = opts.delay
	if opts.list {
		opts.historyPath = ""
	}
	history, err := epubexport.OpenHistory(opts.historyPath, opts.out)
	if err != nil {
		return fmt.Errorf("打开导出记录: %w", err)
	}
	defer history.Close()
	if opts.historyPath != "" && opts.bookID == "" {
		log.Printf("使用导出记录 %s", opts.historyPath)
	}

	if opts.bookID != "" {
		b, err := src.details(ctx, opts.bookID)
		if err != nil {
			return err
		}
		if opts.list {
			fmt.Printf("%s\t%s\t%s\t%s\n", b.id, b.category, b.title, b.author)
			return nil
		}
		return exportBook(ctx, src, opts.out, b, history)
	}
	if opts.list {
		_, err := walkCompleted(ctx, src, opts, func(entry book) error {
			fmt.Printf("%s\t%s\t%s\t%s\n", entry.id, entry.category, entry.title, entry.author)
			return nil
		}, nil)
		return err
	}

	slots := make(chan struct{}, opts.workers)
	var workers sync.WaitGroup
	var failures atomic.Int64
	total, err := walkCompleted(ctx, src, opts, func(entry book) error {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		workers.Go(func() {
			defer func() { <-slots }()
			b, err := src.details(ctx, entry.id)
			if err == nil {
				err = exportBook(ctx, src, opts.out, b, history)
			}
			if err != nil {
				failures.Add(1)
				log.Printf("书籍 %s《%s》失败: %v", entry.id, entry.title, err)
			}
		})
		return nil
	}, func(id string) bool {
		return history.CompletedPath(id) != ""
	})
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if failed := failures.Load(); failed > 0 {
		err = errors.Join(err, fmt.Errorf("处理 %d 本，失败 %d 本", total, failed))
	}
	return err
}

func walkCompleted(ctx context.Context, src *source, opts options, visit func(book) error, skip func(string) bool) (int, error) {
	seen := make(map[string]bool)
	page := "/wanjie/"
	if opts.startPage > 1 {
		page += strconv.Itoa(opts.startPage) + ".html"
	}
	for pageNo := opts.startPage; page != ""; pageNo++ {
		entries, next, err := src.completed(ctx, page)
		if err != nil {
			return len(seen), fmt.Errorf("读取完本榜 %s: %w", page, err)
		}
		if next != "" && next != fmt.Sprintf("/wanjie/%d.html", pageNo+1) {
			return len(seen), fmt.Errorf("完本榜 %s 的下一页链接异常: %s", page, next)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return len(seen), err
			}
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
		page = next
	}
	return len(seen), ctx.Err()
}

func exportBook(ctx context.Context, src *source, out string, b book, history *epubexport.History) error {
	dir := filepath.Join(out, epubexport.SafeName(b.category))
	target := filepath.Join(dir, epubexport.SafeName(b.title)+" ["+b.id+"].epub")
	if info, err := os.Stat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("输出路径已被占用: %s", target)
		}
		//log.Printf("跳过已有文件 %s", target)
		return history.Record(b.id, target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	//log.Printf("导出《%s》 · %s", b.title, b.category)
	toc, err := src.catalog(ctx, b)
	if err != nil {
		return err
	}
	cover, err := src.get(ctx, b.cover, 8<<20)
	if err != nil {
		return fmt.Errorf("获取封面: %w", err)
	}
	bodies, err := src.volumeBodies(ctx, b, toc)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	missing := len(toc)
	for _, body := range bodies {
		if len(body) > 0 {
			missing--
		}
	}
	log.Printf("《%s》分卷 TXT 已匹配 %d/%d 章，需要网页补抓 %d 章", b.title, len(toc)-missing, len(toc), missing)
	if missing > 0 && err != nil {
		log.Printf("《%s》分卷 TXT 问题: %v", b.title, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	meta := epubexport.Book{
		Title: b.title, Author: b.author, Category: b.category,
		Source: "https://www.suduguu.com/" + b.id + "/", Description: b.description,
	}
	titles := make([]string, len(toc))
	for i, ch := range toc {
		titles[i] = ch.title
	}
	fetched := 0
	err = epubexport.Write(target, meta, titles, cover, func(i int) ([]string, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var paragraphs []string
		if i < len(bodies) {
			paragraphs = bodies[i]
		}
		if len(paragraphs) == 0 {
			if fetched%50 == 0 {
				log.Printf("《%s》正在补抓第 %d/%d 章（目录位置 %d/%d）", b.title, fetched+1, missing, i+1, len(toc))
			}
			fetched++
			var err error
			paragraphs, err = src.chapterBody(ctx, toc[i])
			if err != nil {
				return nil, err
			}
		}
		for j, text := range paragraphs {
			paragraphs[j] = chapterText.Replace(text)
		}
		return paragraphs, nil
	})
	if err != nil {
		return err
	}
	log.Printf("已保存 %s（%d 章）", target, len(toc))
	return history.Record(b.id, target)
}
