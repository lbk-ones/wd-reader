package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wd-reader/internal/epubexport"
)

const defaultOutput = `F:\bk-books\book-list-ixdzs`

type options struct {
	historyPath               string
	out, id                   string
	startPage, limit, workers int
	list, allowIncomplete     bool
}

func main() {
	opts := options{historyPath: filepath.Join(epubexport.HistoryDir, "ixdzs-export.jsonl")}
	flag.StringVar(&opts.out, "out", defaultOutput, "EPUB 输出目录")
	flag.StringVar(&opts.id, "book", "", "只处理指定书籍 ID（详情页网址中的数字）")
	flag.IntVar(&opts.startPage, "start-page", 1, "每个分类从第几页开始")
	flag.IntVar(&opts.limit, "limit", 0, "本次最多处理多少本（0 表示全部）")
	flag.IntVar(&opts.workers, "workers", 20, "同时导出的书籍数量")
	flag.BoolVar(&opts.list, "list", false, "只列出书籍，不生成 EPUB")
	flag.BoolVar(&opts.allowIncomplete, "allow-incomplete", true, "跳过 TXT 中缺正文的章节并标记正文不全；设为 false 时尝试从网页补抓")
	delay := flag.Duration("delay", 200*time.Millisecond, "所有书籍共享的请求间隔")
	flag.Parse()
	if flag.NArg() != 0 || opts.startPage < 1 || opts.limit < 0 || opts.workers < 1 || *delay < 0 ||
		(opts.id != "" && !bookPath.MatchString("/read/"+opts.id+"/")) {
		flag.Usage()
		os.Exit(2)
	}
	if err := execute(opts, *delay); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func execute(opts options, delay time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	src, stopPacing := newSource(delay)
	defer stopPacing()
	return run(ctx, src, opts)
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
			if err != nil {
				failures.Add(1)
				log.Printf("书籍 %s《%s》失败: %v", entry.id, entry.title, err)
			}
		})
		return nil
	}, func(id string) bool {
		path := history.CompletedPath(id)
		return path != "" && (opts.allowIncomplete || !strings.HasSuffix(path, "（正文不全）.epub"))
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

func walkCompleted(ctx context.Context, src *source, opts options, visit func(listing) error, skip func(string) bool) (int, error) {
	categories, err := src.categories(ctx)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]bool)
	for page := opts.startPage; len(categories) > 0; page++ {
		active := categories[:0]
		for _, category := range categories {
			entries, next, err := src.completed(ctx, category, page)
			if err != nil {
				return len(seen), fmt.Errorf("分类 %s 第 %d 页: %w", category, page, err)
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
			if next {
				active = append(active, category)
			}
		}
		categories = active
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
	//log.Printf("导出《%s》 · %s，正在下载 ZIP", b.Title, b.Category)
	text, err := src.zipText(ctx, b)
	if err != nil {
		return err
	}
	bodies, splitErr := splitTXT(text, b)
	if bodies == nil && splitErr != nil {
		return splitErr
	}
	if splitErr != nil {
		if titles, nativeBodies := nativeTXT(text, b); len(titles) > 0 {
			log.Printf("《%s》网页和 ZIP 目录不同，使用 ZIP 原始章节（%d 章）", b.Title, len(titles))
			b.titles, b.paths, bodies = titles, make([]string, len(titles)), nativeBodies
			splitErr = nil
		} else {
			log.Printf("《%s》ZIP 正文部分对齐: %v", b.Title, splitErr)
		}
	}
	titles, paths, bodies := uniqueChapters(b, bodies)
	if len(titles) == 0 {
		return fmt.Errorf("TXT 未找到可导出的章节")
	}
	missing := 0
	for i, body := range bodies {
		if len(body) > 0 {
			continue
		}
		missing++
		if !opts.allowIncomplete {
			paragraphs, err := src.chapterBody(ctx, paths[i], titles[i])
			if err != nil {
				return fmt.Errorf("TXT 第 %d 章《%s》缺正文，补抓失败: %w", i+1, titles[i], err)
			}
			bodies[i] = paragraphs
		}
	}
	if opts.allowIncomplete && missing > 0 {
		if missing == len(titles) {
			return errors.Join(fmt.Errorf("ZIP 中没有可用的章节正文，可用 -allow-incomplete=false 尝试网页补抓"), splitErr)
		}
		log.Printf("《%s》TXT 缺少 %d/%d 章正文，另存可用章节", b.Title, missing, len(titles))
		target = strings.TrimSuffix(target, ".epub") + "（正文不全）.epub"
		if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
			log.Printf("跳过已有文件 %s", target)
			return history.Record(b.id, target)
		}
		b.Description = append([]string{fmt.Sprintf("来源 TXT 缺少 %d/%d 章正文，本文件仅收录有正文的章节。", missing, len(titles))}, b.Description...)
		validTitles, validBodies := titles[:0], bodies[:0]
		for i, body := range bodies {
			if len(body) > 0 {
				validTitles = append(validTitles, titles[i])
				validBodies = append(validBodies, body)
			}
		}
		titles, bodies = validTitles, validBodies
	}
	cover, _, err := src.get(ctx, b.cover, b.Source)
	if errors.Is(err, fs.ErrNotExist) {
		log.Printf("《%s》原封面返回 HTTP 404，使用默认封面", b.Title)
		cover, err = epubexport.DefaultCover()
	}
	if err != nil {
		return fmt.Errorf("获取封面: %w", err)
	}
	cover, err = convertCover(cover)
	if err != nil {
		return fmt.Errorf("转换封面: %w", err)
	}
	if !supportedCover(cover) {
		log.Printf("《%s》原封面未返回图片，使用默认封面", b.Title)
		cover, err = epubexport.DefaultCover()
		if err != nil {
			return fmt.Errorf("生成默认封面: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	err = epubexport.Write(target, b.Book, titles, cover, func(i int) ([]string, error) {
		return bodies[i], ctx.Err()
	})
	if err != nil {
		return err
	}
	log.Printf("已保存 %s（%d 章）", target, len(titles))
	return history.Record(b.id, target)
}

func uniqueChapters(b book, bodies [][]string) ([]string, []string, [][]string) {
	var titles, paths []string
	var uniqueBodies [][]string
	type chapterKey struct{ path, title string }
	seen := make(map[chapterKey][]int)
	for i, path := range b.paths {
		if path == "" && len(bodies[i]) == 0 {
			continue
		}
		key := chapterKey{path, b.titles[i]}
		duplicate := false
		for _, j := range seen[key] {
			switch {
			case len(uniqueBodies[j]) == 0:
				uniqueBodies[j] = bodies[i]
				duplicate = true
			case len(bodies[i]) == 0 || slices.Equal(uniqueBodies[j], bodies[i]):
				duplicate = true
			}
			if duplicate {
				break
			}
		}
		if duplicate {
			continue
		}
		if path != "" {
			// Some URLs and titles are reused for distinct chapters in the TXT.
			seen[key] = append(seen[key], len(titles))
		}
		titles = append(titles, b.titles[i])
		paths = append(paths, path)
		uniqueBodies = append(uniqueBodies, bodies[i])
	}
	return titles, paths, uniqueBodies
}
