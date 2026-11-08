package main

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
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
	bookPath     = regexp.MustCompile("^/txt/([0-9]+)[.]html$")
	downloadPath = regexp.MustCompile("^/txt/dl-[0-9]+-([0-9]+)[.]html$")
	readingPath  = regexp.MustCompile("^/read/([0-9]+)/([0-9]+)/([0-9]+)[.]html$")
	progress     = regexp.MustCompile("写作进度[：:]\\s*(\\S+)")
	pageSuffix   = regexp.MustCompile("^(.+?)[（(]([0-9]+)/([0-9]+)[）)]$")
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if delay := time.Until(s.lastRequest.Add(s.delay)); delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.lastRequest = time.Now()
	return nil
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
			err = fmt.Errorf("%s: HTTP %d", u.Path, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
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
	data, _, err := s.get(ctx, path)
	if err != nil {
		return nil, err
	}
	// 部分 .jpg 封面实际为 AVIF，转为 EPUB 核心图片格式。
	if len(data) < 12 || (string(data[4:12]) != "ftypavif" && string(data[4:12]) != "ftypavis") {
		return data, nil
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
			return nil, false, fmt.Errorf("%s 完本条目缺少书名或链接", path)
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
	if b.Title == "" || b.Author == "" || b.Category == "" || b.cover == "" || len(b.Description) == 0 {
		return book{}, fmt.Errorf("%s 的书名、作者、分类、封面或简介缺失", path)
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
	var toc []chapter
	pageNo, totalPages := 0, 0
	class := ""
	for n := range list.Descendants() {
		if n.Type != html.ElementNode || n.Data != "a" {
			continue
		}
		href, title := attr(n, "href"), textOf(n)
		m := readingPath.FindStringSubmatch(href)
		pageNo++
		if m == nil || m[2] != id || m[3] != strconv.Itoa(pageNo) || title == "" || (class != "" && m[1] != class) {
			return nil, fmt.Errorf("目录第 %d 个条目缺失或异常: %s", pageNo, href)
		}
		class = m[1]
		part, total := 1, 0
		if suffix := pageSuffix.FindStringSubmatch(title); suffix != nil {
			title = strings.TrimSpace(suffix[1])
			part, _ = strconv.Atoi(suffix[2])
			total, _ = strconv.Atoi(suffix[3])
			if part < 1 || total < part {
				return nil, fmt.Errorf("章节分页异常: %s", href)
			}
		}
		if part == 1 {
			if totalPages > 0 && len(toc[len(toc)-1].pages) != totalPages {
				return nil, fmt.Errorf("上一章分页不完整: %s", href)
			}
			toc = append(toc, chapter{title: title, pages: []string{href}})
			totalPages = total
			continue
		}
		if len(toc) == 0 {
			return nil, fmt.Errorf("章节分页缺少首页: %s", href)
		}
		current := &toc[len(toc)-1]
		if titleKey(current.title) != titleKey(title) || len(current.pages)+1 != part || (totalPages > 0 && totalPages != total) {
			return nil, fmt.Errorf("章节分页顺序异常: %s", href)
		}
		totalPages = total
		current.pages = append(current.pages, href)
	}
	if len(toc) == 0 || (totalPages > 0 && len(toc[len(toc)-1].pages) != totalPages) {
		return nil, fmt.Errorf("章节目录为空或最后一章分页不完整")
	}
	return toc, nil
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
