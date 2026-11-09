package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/avif"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"wd-reader/internal/epubexport"
)

const site = "https://www.ijjxsxzw.com"

var (
	bookPath        = regexp.MustCompile("^/txt/([0-9]+)[.]html$")
	downloadPath    = regexp.MustCompile("^/txt/dl-[0-9]+-([0-9]+)[.]html$")
	readingPath     = regexp.MustCompile("^/read/([0-9]+)/([0-9]+)/([0-9]+)[.]html$")
	progress        = regexp.MustCompile("写作进度[：:]\\s*(\\S+)")
	pageSuffix      = regexp.MustCompile("^(.+?)[（(]([0-9]+)/([0-9]+)[）)]$")
	errEmptyCatalog = errors.New("章节目录为空")
	errRateLimited  = errors.New("HTTP 429 请求受限")
)

type book struct {
	epubexport.Book
	id, cover, download string
}

type chapter struct {
	title string
	pages []string
}

type source struct {
	base        *url.URL
	client      *http.Client
	delay       time.Duration
	mu          sync.Mutex
	lastRequest time.Time
	retryAt     time.Time
}

func newSource(delay time.Duration) *source {
	base, _ := url.Parse(site)
	s := &source{base: base, delay: delay}
	s.client = &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !s.allowed(req.URL) {
				return fmt.Errorf("意外重定向: %s", req.URL.Host)
			}
			return nil
		},
	}
	return s
}

func (s *source) allowed(u *url.URL) bool {
	return u.User == nil && (u.Scheme == s.base.Scheme && u.Host == s.base.Host ||
		u.Scheme == "https" && (u.Host == "image.jjjjxsw.com" || u.Host == "jjxswshuku.com"))
}

func (s *source) wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		next := s.lastRequest.Add(s.delay)
		if s.retryAt.After(next) {
			next = s.retryAt
		}
		delay := time.Until(next)
		if delay <= 0 {
			s.lastRequest = time.Now()
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (s *source) rateLimited(path, retryAfter string) error {
	now := time.Now()
	retryAt := now.Add(time.Minute)
	retryAfter = strings.TrimSpace(retryAfter)
	if seconds, err := strconv.ParseUint(retryAfter, 10, 32); err == nil {
		retryAt = now.Add(time.Duration(seconds) * time.Second)
	} else if date, err := http.ParseTime(retryAfter); err == nil {
		retryAt = date
	}
	s.mu.Lock()
	if retryAt.After(s.retryAt) {
		s.retryAt = retryAt
	}
	wait := max(time.Until(s.retryAt), 0).Round(time.Second)
	s.mu.Unlock()
	return fmt.Errorf("%s: %w，共享冷却 %s", path, errRateLimited, wait)
}

func (s *source) get(ctx context.Context, href string) ([]byte, string, error) {
	u, err := s.base.Parse(href)
	if err != nil {
		return nil, "", err
	}
	if !s.allowed(u) {
		return nil, "", fmt.Errorf("来源以外的地址: %s", u)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "wd-reader/jjxs-epub")
	req.Header.Set("Referer", s.base.String()+"/")
	const limit = 8 << 20
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		if err := s.wait(ctx); err != nil {
			return nil, "", err
		}
		res, requestErr := s.client.Do(req)
		if requestErr != nil {
			err = requestErr
			continue
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			if res.StatusCode == http.StatusTooManyRequests {
				return nil, "", s.rateLimited(u.Path, res.Header.Get("Retry-After"))
			}
			err = fmt.Errorf("%s: HTTP %d", u.Path, res.StatusCode)
			if res.StatusCode >= 500 {
				continue
			}
			return nil, "", err
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, limit+1))
		res.Body.Close()
		if readErr != nil {
			err = readErr
			continue
		}
		if len(data) > limit {
			return nil, "", fmt.Errorf("%s 超过响应大小限制", u.Path)
		}
		return data, res.Header.Get("Content-Type"), nil
	}
	return nil, "", err
}

func (s *source) page(ctx context.Context, path string) (*html.Node, error) {
	data, contentType, err := s.get(ctx, path)
	if err != nil {
		return nil, err
	}
	reader, err := charset.NewReader(bytes.NewReader(data), contentType)
	if err != nil {
		return nil, err
	}
	return html.Parse(reader)
}

func (s *source) coverImage(ctx context.Context, path string) ([]byte, error) {
	if path == "" {
		return epubexport.DefaultCover()
	}
	data, err := s.fetchCover(ctx, path)
	if err == nil {
		return data, nil
	}
	if errors.Is(err, errRateLimited) {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	log.Printf("封面 %s 不可用，使用默认封面: %v", path, err)
	return epubexport.DefaultCover()
}

func (s *source) fetchCover(ctx context.Context, path string) ([]byte, error) {
	data, _, err := s.get(ctx, path)
	if err != nil {
		return nil, err
	}
	// 部分 .jpg 封面实际为 AVIF，转为 EPUB 核心图片格式。
	if len(data) < 12 || (string(data[4:12]) != "ftypavif" && string(data[4:12]) != "ftypavis") {
		switch kind := http.DetectContentType(data); kind {
		case "image/jpeg", "image/png", "image/gif", "image/webp":
			return data, nil
		default:
			return nil, fmt.Errorf("无法识别封面图片: %s", kind)
		}
	}
	config, err := avif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width > 4096 || config.Height > 4096 {
		return nil, fmt.Errorf("AVIF 封面尺寸超出限制")
	}
	image, err := avif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image); err != nil {
		return nil, err
	}
	return pngData.Bytes(), nil
}

type listing struct{ id, title string }

func listPath(section, page int) string {
	return fmt.Sprintf("/txt/toplist-p_%d-c_%d-n_0-l_10-t_6-p_1-s_0-q_0-r_1-m_0.html", page-1, section)
}

func (s *source) completed(ctx context.Context, section, page int) ([]listing, bool, error) {
	path := listPath(section, page)
	doc, err := s.page(ctx, path)
	if err != nil {
		return nil, false, err
	}
	var entries []listing
	var cards int
	for n := range doc.Descendants() {
		if !hasClass(n, "listbg") {
			continue
		}
		cards++
		status := progress.FindStringSubmatch(textOf(find(n, "span", "mainGreen")))
		if status == nil || !completeStatus(status[1]) {
			continue
		}
		a := find(find(n, "span", "title"), "a", "")
		m := bookPath.FindStringSubmatch(attr(a, "href"))
		title := textOf(a)
		if m == nil || title == "" {
			log.Printf("%s 跳过无法识别的完本条目《%s》: %s", path, title, attr(a, "href"))
			continue
		}
		entries = append(entries, listing{id: m[1], title: title})
	}
	pager := find(doc, "div", "pager")
	if cards == 0 || pager == nil {
		return nil, false, fmt.Errorf("%s 未识别书目或分页", path)
	}
	for n := range pager.Descendants() {
		if n.Type == html.ElementNode && n.Data == "a" && textOf(n) == "下一页" {
			if attr(n, "href") != listPath(section, page+1) {
				return nil, false, fmt.Errorf("%s 下一页链接异常: %s", path, attr(n, "href"))
			}
			return entries, true, nil
		}
	}
	return entries, false, nil
}

func completeStatus(status string) bool {
	switch status {
	case "已完结", "正文完", "番外完", "全本校对":
		return true
	default:
		return false
	}
}

func (s *source) details(ctx context.Context, id string) (book, error) {
	path := "/txt/" + id + ".html"
	doc, err := s.page(ctx, path)
	if err != nil {
		return book{}, err
	}
	detail := find(doc, "div", "detail")
	if detail == nil {
		return book{}, fmt.Errorf("%s 缺少书籍详情", path)
	}
	kv := find(detail, "div", "kv")
	if !completeStatus(textOf(find(kv, "span", "sfwj"))) {
		return book{}, fmt.Errorf("%s 未标记为完结", path)
	}
	title := textOf(find(detail.Parent, "h3", ""))
	if inner, ok := strings.CutPrefix(title, "《"); ok {
		if inner, ok = strings.CutSuffix(inner, "》"); ok {
			title = inner
		}
	}
	b := book{
		id: id, cover: attr(find(find(detail, "div", "pic"), "img", ""), "src"),
		Book: epubexport.Book{
			Title: title, Source: s.base.String() + path,
			Description: textLines(find(find(doc, "article", "intro-panel"), "div", "desc")),
		},
	}
	if kv != nil {
		for n := range kv.Descendants() {
			if n.Type != html.ElementNode || n.Data != "p" {
				continue
			}
			label := textOf(find(n, "strong", ""))
			value := strings.TrimSpace(strings.TrimPrefix(textOf(n), label))
			switch label {
			case "书籍作者：":
				b.Author = value
			case "书籍分类：":
				b.Category = value
			}
		}
	}
	for n := range doc.Descendants() {
		href := attr(n, "href")
		if m := downloadPath.FindStringSubmatch(href); m != nil && m[1] == id {
			b.download = href
			break
		}
	}
	if b.Title == "" || b.Category == "" {
		return book{}, fmt.Errorf("%s 的书名或分类缺失", path)
	}
	if b.Author == "" {
		// 部分详情页的作者栏为空，但 HTML 标题仍包含作者。
		if metadata, ok := strings.CutPrefix(textOf(find(doc, "title", "")), b.Title+"txt下载_"); ok {
			author, _, _ := strings.Cut(metadata, "_"+b.Category+"_")
			if author != metadata {
				b.Author = strings.TrimSpace(author)
			}
		}
		if b.Author == "" {
			b.Author = "未知作者"
		}
	}
	return b, nil
}

func (s *source) catalog(ctx context.Context, id string) ([]chapter, error) {
	doc, err := s.page(ctx, "/read/"+id+"/")
	if err != nil {
		return nil, err
	}
	list := find(doc, "ul", "chapter-list")
	if list == nil {
		return nil, fmt.Errorf("没有找到在线章节目录")
	}
	var pages []chapter
	class := ""
	for n := range list.Descendants() {
		if n.Type != html.ElementNode || n.Data != "a" {
			continue
		}
		href, title := attr(n, "href"), textOf(n)
		m := readingPath.FindStringSubmatch(href)
		pageNo := len(pages) + 1
		if m == nil || m[2] != id || m[3] != strconv.Itoa(pageNo) || title == "" || (class != "" && m[1] != class) {
			return nil, fmt.Errorf("目录第 %d 个条目缺失或异常: %s", pageNo, href)
		}
		class = m[1]
		pages = append(pages, chapter{title: title, pages: []string{href}})
	}
	if len(pages) == 0 {
		return nil, errEmptyCatalog
	}
	return mergeChapterPages(pages), nil
}

func mergeChapterPages(pages []chapter) []chapter {
	// 章名也可能包含加更序号或字数。只有连续同名且页码齐全时才合并。
	var toc []chapter
	for i := 0; i < len(pages); {
		ch := pages[i]
		title, part, total := chapterPageTitle(ch.title)
		if part == 1 && i+1 < len(pages) {
			nextTitle, nextPart, nextTotal := chapterPageTitle(pages[i+1].title)
			if nextPart == 2 && (total == 0 || total == nextTotal) &&
				nextTotal <= len(pages)-i && titleKey(title) == titleKey(nextTitle) {
				end := i + 2
				for ; end < i+nextTotal; end++ {
					label, number, count := chapterPageTitle(pages[end].title)
					if number != end-i+1 || count != nextTotal || titleKey(label) != titleKey(title) {
						break
					}
				}
				if end == i+nextTotal {
					ch.title = title
					for _, page := range pages[i+1 : end] {
						ch.pages = append(ch.pages, page.pages...)
					}
					i = end - 1
				}
			}
		}
		toc = append(toc, ch)
		i++
	}
	return toc
}

func chapterPageTitle(title string) (string, int, int) {
	if suffix := pageSuffix.FindStringSubmatch(title); suffix != nil {
		part, partErr := strconv.Atoi(suffix[2])
		total, totalErr := strconv.Atoi(suffix[3])
		if partErr == nil && totalErr == nil && part > 0 && total >= part {
			return strings.TrimSpace(suffix[1]), part, total
		}
	}
	return title, 1, 0
}

func (s *source) chapterBody(ctx context.Context, ch chapter) ([]string, error) {
	var body []string
	for _, path := range ch.pages {
		doc, err := s.page(ctx, path)
		if err != nil {
			return nil, err
		}
		lines := textLines(find(doc, "div", "#view_content_txt"))
		if len(lines) == 0 {
			return nil, fmt.Errorf("%s 正文为空", path)
		}
		body = append(body, stripHeadings(lines, ch.title)...)
	}
	return body, nil
}

func attr(n *html.Node, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, item := range strings.Fields(attr(n, "class")) {
		if item == class {
			return true
		}
	}
	return false
}

func find(root *html.Node, tag, selector string) *html.Node {
	if root != nil {
		for n := range root.Descendants() {
			if n.Type != html.ElementNode || n.Data != tag {
				continue
			}
			if selector == "" || hasClass(n, selector) || selector == "#"+attr(n, "id") {
				return n
			}
		}
	}
	return nil
}
