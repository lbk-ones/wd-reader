// merge-books selects and copies EPUB editions into the flat bk-books taxonomy.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"wd-reader/internal/epubexport"
)

const defaultRoot = `F:\bk-books`

func main() {
	root := flag.String("root", defaultRoot, "书库根目录")
	apply := flag.Bool("apply", false, "执行复制，默认只预览")
	show := flag.Bool("show-duplicates", true, "打印每组重复书的入选和淘汰路径")
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*root, *apply, *show); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(root string, apply, show bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	groups, scanned, issues, err := scanSources(root)
	if err != nil {
		return err
	}
	out := filepath.Join(root, "all")
	existing, occupied, err := previousCopies(out)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	choices := rankEditions(keys, groups)
	totals := make(map[string]int)
	copied, present, planned, duplicates := 0, 0, 0, 0
	for i, key := range keys {
		group := groups[key]
		winner, q := choices[i].winner, choices[i].quality
		if winner.path == "" {
			issues++
			continue
		}
		duplicates += len(group) - 1
		totals[winner.category]++
		dir := filepath.Join(out, winner.category)
		name := winner.title
		if winner.author != "" {
			name += " - " + winner.author
		}
		name = epubexport.SafeName(name)
		target := filepath.Join(dir, name+".epub")
		paths := existing[key]
		if len(paths) == 0 {
			for suffix := 2; occupied[strings.ToLower(target)]; suffix++ {
				target = filepath.Join(dir, fmt.Sprintf("%s - %d.epub", name, suffix))
			}
			occupied[strings.ToLower(target)] = true
		}
		if show && len(group) > 1 {
			fmt.Printf("保留 %s -> %s（正文 %d 字，目录 %d 项）\n", winner.path, target, q.text, q.chapters)
			for _, book := range group {
				if book.path != winner.path {
					fmt.Printf("  重复 %s\n", book.path)
				}
			}
		}
		if len(paths) > 0 {
			if err := checkPreviousCopy(paths, winner.path, dir); err != nil {
				log.Print(err)
				issues++
			} else {
				present++
			}
			continue
		}
		planned++
		if apply {
			if err := copyEdition(winner.path, target); err != nil {
				log.Printf("复制失败 %s: %v", target, err)
				issues++
			} else {
				copied++
			}
		}
	}
	labels := make([]string, 0, len(totals))
	for category := range totals {
		labels = append(labels, category)
	}
	slices.Sort(labels)
	for _, category := range labels {
		fmt.Printf("%s\t%d\n", category, totals[category])
	}
	fmt.Printf("扫描 %d 本，重复 %d 个版本，已存在 %d 本，待复制 %d 本，已复制 %d 本，异常 %d 项\n",
		scanned, duplicates, present, planned-copied, copied, issues)
	if issues > 0 {
		return fmt.Errorf("%d 项需检查，详见日志", issues)
	}
	return nil
}

func scanSources(root string) (map[string][]edition, int, int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, 0, err
	}
	index := categoryIndex()
	groups := make(map[string][]edition)
	scanned, issues, sources := 0, 0, 0
	for _, dir := range entries {
		name := strings.ToLower(dir.Name())
		if !dir.IsDir() || (!strings.HasPrefix(name, "book-") && !strings.HasPrefix(name, "books-")) {
			continue
		}
		sources++
		base := filepath.Join(root, dir.Name())
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("无法扫描 %s: %v", path, err)
				issues++
				return nil
			}
			if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".epub") {
				return nil
			}
			relative, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			categoryName, _, inSubdir := strings.Cut(relative, string(filepath.Separator))
			if !inSubdir {
				return nil
			}
			scanned++
			if scanned%5000 == 0 {
				log.Printf("已扫描 %d 本 EPUB", scanned)
			}
			category := index[categoryName]
			if category == "" {
				category = "未分类"
			}
			book, err := readEdition(path, category)
			if err != nil {
				log.Printf("无法读取 %s: %v", path, err)
				issues++
			} else {
				groups[book.key] = append(groups[book.key], book)
			}
			return nil
		})
		if err != nil {
			return nil, scanned, issues, err
		}
	}
	if sources == 0 {
		return nil, 0, 0, fmt.Errorf("%s 下没有 book-* 或 books-* 目录", root)
	}
	return groups, scanned, issues, nil
}

type choice struct {
	winner  edition
	quality quality
}

func rankEditions(keys []string, groups map[string][]edition) []choice {
	result := make([]choice, len(keys))
	workers := min(len(keys), runtime.GOMAXPROCS(0), 8)
	var wg sync.WaitGroup
	var compared atomic.Int64
	for worker := range workers {
		wg.Go(func() {
			for i := worker; i < len(keys); i += workers {
				group := groups[keys[i]]
				result[i] = bestEdition(group)
				if len(group) > 1 {
					if count := compared.Add(1); count%500 == 0 {
						log.Printf("已比较 %d 组重复书", count)
					}
				}
			}
		})
	}
	wg.Wait()
	return result
}

func bestEdition(group []edition) choice {
	if len(group) == 1 {
		return choice{winner: group[0]}
	}
	var winner, classified edition
	var best, bestClassified quality
	for _, book := range group {
		q, err := assess(book)
		if err != nil {
			log.Printf("忽略无法读取正文的重复版本 %s: %v", book.path, err)
			continue
		}
		// 输入按路径扫描，质量相同保留第一个版本，结果可重复。
		if winner.path == "" || q.betterThan(best) {
			winner, best = book, q
		}
		if book.category != "未分类" && (classified.path == "" || q.betterThan(bestClassified)) {
			classified, bestClassified = book, q
		}
	}
	if winner.category == "未分类" && classified.path != "" {
		winner.category = classified.category
	}
	return choice{winner: winner, quality: best}
}

func previousCopies(out string) (map[string][]string, map[string]bool, error) {
	files := make(map[string][]string)
	occupied := make(map[string]bool)
	info, err := os.Lstat(out)
	if errors.Is(err, fs.ErrNotExist) {
		return files, occupied, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("输出路径不是普通目录: %s", out)
	}
	err = filepath.WalkDir(out, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		occupied[strings.ToLower(path)] = true
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".epub") {
			book, err := readEdition(path, "")
			if err != nil {
				log.Printf("无法核对已有 EPUB %s: %v", path, err)
			} else {
				files[book.key] = append(files[book.key], path)
			}
		}
		return nil
	})
	return files, occupied, err
}

func checkPreviousCopy(paths []string, source, dir string) error {
	if len(paths) != 1 || !strings.EqualFold(filepath.Dir(paths[0]), dir) {
		return fmt.Errorf("已有同一书籍标识的输出，请核对目录或重复文件: %s", strings.Join(paths, "、"))
	}
	a, err := fileDigest(source)
	if err != nil {
		return err
	}
	b, err := fileDigest(paths[0])
	if err != nil {
		return err
	}
	if a != b {
		return fmt.Errorf("已有输出与本次入选版本不同: %s（新版本 %s）", paths[0], source)
	}
	return nil
}

func copyEdition(source, target string) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, path := range []string{filepath.Dir(dir), dir} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("输出路径不是普通目录: %s", path)
		}
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(dir, ".merge-books-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, copyErr := io.Copy(tmp, in)
	if err := errors.Join(copyErr, tmp.Close()); err != nil {
		return err
	}
	// 只有复制完整后才发布 EPUB，若目标已存在则由系统拒绝覆盖。
	return os.Link(tmp.Name(), target)
}
