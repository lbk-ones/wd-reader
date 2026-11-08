package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/epubexport"
)

const site = "https://ixdzs8.com"

var (
	bookPath     = regexp.MustCompile("^/read/([1-9][0-9]*)/$")
	categoryPath = regexp.MustCompile("^/sort/([0-9]+)/$")
	volumeLabel  = regexp.MustCompile("^(?:第[0-9零〇一二三四五六七八九十百千两]+[卷部篇]|正文|番外)(?:$|[ 　：:、])")
	chapterLabel = regexp.MustCompile(chapterdetect.DefaultPattern)
	zipHost      = regexp.MustCompile("^down[0-9]+[.]ixdzs8[.]com$")
	imgHost      = regexp.MustCompile("^img[0-9]*[.]ixdzs8?[.]com$")
)

type listing struct{ id, title string }

type book struct {
	epubexport.Book
	id, cover, download string
	titles, paths       []string
}

type source struct {
	base   *url.URL
	client *http.Client
}

type pacedTransport struct {
	ticks <-chan time.Time
}

func (p pacedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-p.ticks:
		return http.DefaultTransport.RoundTrip(req)
	}
}

func newSource(delay time.Duration) (*source, func()) {
	base, _ := url.Parse(site)
	s := &source{base: base}
	s.client = &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !s.allowed(req.URL) {
				return fmt.Errorf("意外重定向: %s", req.URL.Host)
			}
			return nil
		},
	}
	if delay > 0 {
		ticker := time.NewTicker(delay)
		s.client.Transport = pacedTransport{ticks: ticker.C}
		return s, ticker.Stop
	}
	return s, func() {}
}

func (s *source) allowed(u *url.URL) bool {
	return u.User == nil && (u.Scheme == s.base.Scheme && u.Host == s.base.Host ||
		u.Scheme == "https" && (zipHost.MatchString(u.Host) || imgHost.MatchString(u.Host)))
}

func (s *source) get(ctx context.Context, path, referer string) ([]byte, string, error) {
	u, err := s.base.Parse(path)
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
	req.Header.Set("User-Agent", "wd-reader/ixdzs-epub")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	const limit = 8 << 20
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		res, requestErr := s.client.Do(req)
		if requestErr != nil {
			err = requestErr
			continue
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			if res.StatusCode == http.StatusNotFound {
				return nil, "", fmt.Errorf("%s: HTTP %d: %w", u.Path, res.StatusCode, fs.ErrNotExist)
			}
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
	data, contentType, err := s.get(ctx, path, "")
	if err != nil {
		return nil, err
	}
	reader, err := charset.NewReader(bytes.NewReader(data), contentType)
	if err != nil {
		return nil, err
	}
	return html.Parse(reader)
}

func (s *source) categories(ctx context.Context) ([]string, error) {
	doc, err := s.page(ctx, "/sort/1/")
	if err != nil {
		return nil, err
	}
	filter := find(doc, func(n *html.Node) bool {
		return n.Data == "div" && attr(n, "class") == "s-box" &&
			find(n, func(child *html.Node) bool { return categoryPath.MatchString(attr(child, "href")) }) != nil
	})
	if filter == nil {
		return nil, fmt.Errorf("未识别分类筛选栏")
	}
	var ids []string
	seen := make(map[string]bool)
	for n := range filter.Descendants() {
		if m := categoryPath.FindStringSubmatch(attr(n, "href")); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids, nil
}

func completedPath(category string, page int) string {
	return fmt.Sprintf("/sort/%s/index-0-2-0-%d.html", category, page)
}

func (s *source) completed(ctx context.Context, category string, page int) ([]listing, bool, error) {
	path := completedPath(category, page)
	//path := "/sort/1/index-0-2-0-1.html"
	doc, err := s.page(ctx, path)
	if err != nil {
		return nil, false, err
	}
	list := findClass(doc, "ul", "u-list")
	pager := findClass(doc, "div", "pagei")
	if list == nil || pager == nil {
		return nil, false, fmt.Errorf("%s 未识别完结书单或分页", path)
	}
	var entries []listing
	for li := list.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		if textOf(findClass(li, "span", "end")) != "已完结" {
			continue
		}
		heading := findClass(li, "h3", "bname")
		link := find(heading, func(n *html.Node) bool { return n.Data == "a" })
		m := bookPath.FindStringSubmatch(attr(link, "href"))
		title := textOf(link)
		if m == nil || title == "" {
			return nil, false, fmt.Errorf("%s 完结条目缺少书名或链接", path)
		}
		entries = append(entries, listing{id: m[1], title: title})
	}
	next := find(pager, func(n *html.Node) bool {
		return n.Data == "a" && strings.HasSuffix(attr(n, "title"), "下一页")
	})
	if next != nil && attr(next, "href") != completedPath(category, page+1) {
		return nil, false, fmt.Errorf("%s 的下一页链接异常: %s", path, attr(next, "href"))
	}
	return entries, next != nil, nil
}

func (s *source) details(ctx context.Context, id string) (book, error) {
	path := "/read/" + id + "/"
	doc, err := s.page(ctx, path)
	if err != nil {
		return book{}, err
	}
	meta := make(map[string]string)
	var b book
	b.id, b.Source = id, s.base.String()+path
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "meta":
			meta[attr(n, "property")] = epubexport.UnescapeHTML(attr(n, "content"))
		case "a":
			u, err := url.Parse(attr(n, "href"))
			if err == nil && u.Scheme == "https" && zipHost.MatchString(u.Host) &&
				u.Path == "/"+id+".zip" && s.allowed(u) {
				b.download = u.String()
			}
		}
	}
	if meta["og:novel:status"] != "已完结" {
		return book{}, fmt.Errorf("%s 未标记为已完结", path)
	}
	b.Title, b.Author, b.Category = meta["og:novel:book_name"], meta["og:novel:author"], meta["og:novel:category"]
	b.cover = meta["og:image"]
	for line := range strings.SplitSeq(textOf(find(doc, func(n *html.Node) bool { return attr(n, "id") == "intro" })), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			b.Description = append(b.Description, line)
		}
	}
	if b.Title == "" || b.Author == "" || b.Category == "" || b.cover == "" || len(b.Description) == 0 {
		return book{}, fmt.Errorf("%s 的书名、作者、分类、封面或简介缺失", path)
	}
	if b.download == "" {
		return book{}, fmt.Errorf("%s 未找到 ZIP 下载链接", path)
	}

	catalog := find(doc, func(n *html.Node) bool { return attr(n, "id") == "a-list" })
	list := findClass(catalog, "ul", "u-chapter")
	if list == nil {
		return book{}, fmt.Errorf("%s 未找到完整章节目录", path)
	}
	seen := make(map[string]string)
	slots, numberedSlots := 0, true
	hasUnlinkedChapter := false
	for item := list.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "li" {
			continue
		}
		slots++
		n := find(item, func(n *html.Node) bool { return n.Data == "a" })
		if n == nil {
			title := textOf(item)
			switch {
			case volumeLabel.MatchString(title):
			case chapterLabel.MatchString(title):
				b.titles = append(b.titles, title)
				b.paths = append(b.paths, "")
				hasUnlinkedChapter = true
			default:
				numberedSlots = false
			}
			continue
		}
		href, title := attr(n, "href"), textOf(n)
		stem, ok := strings.CutPrefix(href, path+"p")
		num, numeric := strings.CutSuffix(stem, ".html")
		chapter, err := strconv.Atoi(num)
		if !ok || !numeric || err != nil || chapter < 1 || title == "" {
			return book{}, fmt.Errorf("%s 有异常的目录条目: %s", path, href)
		}
		if prior, exists := seen[href]; exists {
			if prior != title {
				return book{}, fmt.Errorf("%s 同一个章节地址对应不同标题: %s", path, href)
			}
		} else {
			numberedSlots = numberedSlots && chapter == slots
		}
		seen[href] = title
		b.titles = append(b.titles, title)
		b.paths = append(b.paths, href)
	}
	if len(b.titles) == 0 {
		return book{}, fmt.Errorf("%s 的目录为空", path)
	}
	if hasUnlinkedChapter && !numberedSlots {
		return book{}, fmt.Errorf("%s 的无链接章节与网页目录顺序不符", path)
	}
	if count := textOf(findClass(doc, "span", "sub-text-r")); count != "" {
		// 有些书把无链接的卷标也计入总数，后续章节地址会按这个位置编号。
		if count != fmt.Sprintf("共%d章", len(b.titles)) && !(numberedSlots && count == fmt.Sprintf("共%d章", slots)) {
			return book{}, fmt.Errorf("%s 的目录数量不符: %s，实际 %d 项", path, count, len(b.titles))
		}
	}
	if href := meta["og:novel:latest_chapter_url"]; href != "" {
		u, err := s.base.Parse(href)
		if err != nil || seen[u.Path] == "" {
			return book{}, fmt.Errorf("%s 的目录缺少最新章节", path)
		}
	}
	return b, nil
}

func (s *source) chapterBody(ctx context.Context, path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("该章节没有网页正文链接")
	}
	doc, err := s.page(ctx, path)
	if err != nil {
		return nil, err
	}
	section := find(findClass(doc, "article", "page-content"), func(n *html.Node) bool { return n.Data == "section" })
	if section == nil {
		return nil, fmt.Errorf("网页章节 %s 未找到正文", path)
	}
	var body []string
	for n := range section.Descendants() {
		if n.Type == html.ElementNode && n.Data == "p" {
			if text := textOf(n); text != "" {
				body = append(body, text)
			}
		}
	}
	if len(body) == 0 || (len(body) == 1 && strings.HasPrefix(body[0], "内容还在处理中")) {
		return nil, fmt.Errorf("网页章节 %s 暂无正文", path)
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

func find(root *html.Node, match func(*html.Node) bool) *html.Node {
	if root != nil {
		for n := range root.Descendants() {
			if n.Type == html.ElementNode && match(n) {
				return n
			}
		}
	}
	return nil
}

func findClass(root *html.Node, tag, class string) *html.Node {
	return find(root, func(n *html.Node) bool {
		if n.Data == tag {
			for _, name := range strings.Fields(attr(n, "class")) {
				if name == class {
					return true
				}
			}
		}
		return false
	})
}

func textOf(n *html.Node) string {
	var text strings.Builder
	if n != nil {
		for child := range n.Descendants() {
			if child.Type == html.TextNode {
				text.WriteString(child.Data)
			} else if child.Type == html.ElementNode && child.Data == "br" {
				text.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(epubexport.UnescapeHTML(text.String()))
}
