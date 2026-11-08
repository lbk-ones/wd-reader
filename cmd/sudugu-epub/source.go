package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/epubexport"
)

var (
	bookPath    = regexp.MustCompile(`^/[1-9][0-9]*/$`)
	chapterPath = regexp.MustCompile(`^/[0-9]+/[0-9]+[.]html$`)
	volumeRange = regexp.MustCompile(`\(([0-9]+)-([0-9]+)章\)[.]txt$`)
)

const volumeTimeout = 3 * time.Minute

type book struct {
	id, title, author, category, cover string
	description                        []string
}

type chapter struct {
	path, title string
}

type source struct {
	base        *url.URL
	client      *http.Client
	http1Client *http.Client
	delay       time.Duration
	requestMu   sync.Mutex
	lastRequest time.Time
}

func newSource() *source {
	base, _ := url.Parse("https://www.suduguu.com")
	client := &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || req.URL.Scheme != base.Scheme || req.URL.Host != base.Host {
				return fmt.Errorf("意外重定向: %s", req.URL)
			}
			return nil
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Protocols = &http.Protocols{}
	transport.Protocols.SetHTTP1(true)
	http1Client := *client
	http1Client.Transport = transport
	http1Client.Timeout = volumeTimeout
	return &source{base: base, client: client, http1Client: &http1Client}
}

func (s *source) waitForRequest(ctx context.Context) error {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	if wait := time.Until(s.lastRequest.Add(s.delay)); wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.lastRequest = time.Now()
	return nil
}

func (s *source) get(ctx context.Context, href string, maxBytes int64) ([]byte, error) {
	u, err := s.base.Parse(href)
	if err != nil {
		return nil, err
	}
	if u.Scheme != s.base.Scheme || u.Host != s.base.Host || u.User != nil {
		return nil, fmt.Errorf("来源以外的链接: %s", u)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36 Edg/157.0.0.0")
	client := s.client
	isVolume := u.Path == "/txt/"
	if isVolume {
		volumeClient := *client
		volumeClient.Timeout = volumeTimeout
		client = &volumeClient
	}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		if err := s.waitForRequest(ctx); err != nil {
			return nil, err
		}

		res, requestErr := client.Do(req)
		if requestErr != nil {
			err = requestErr
			continue
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			err = fmt.Errorf("%s: HTTP %d", u, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
				continue
			}
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
		res.Body.Close()
		if readErr != nil {
			err = fmt.Errorf("读取 %s: %w", u, readErr)
			// 分卷响应中途被取消时，用独立的 HTTP/1.1 连接重下整卷。
			if isVolume && res.ProtoMajor == 2 {
				client = s.http1Client
			}
			continue
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("%s 超过响应大小限制", u)
		}
		return data, nil
	}
	return nil, err
}

func (s *source) page(ctx context.Context, path string) (*html.Node, error) {
	data, err := s.get(ctx, path, 8<<20)
	if err != nil {
		return nil, err
	}
	reader := io.Reader(bytes.NewReader(data))
	if !utf8.Valid(data) {
		reader, err = charset.NewReader(reader, "")
		if err != nil {
			return nil, err
		}
	}
	return html.Parse(reader)
}

func (s *source) completed(ctx context.Context, path string) ([]book, string, error) {
	doc, err := s.page(ctx, path)
	if err != nil {
		return nil, "", err
	}
	var entries []book
	for n := range doc.Descendants() {
		if !hasClass(n, "itemtxt") {
			continue
		}
		link := find(find(n, "h3", ""), "a", "")
		href := attr(link, "href")
		category := completedCategory(n)
		if !bookPath.MatchString(href) || category == "" {
			continue
		}
		entries = append(entries, book{
			id: strings.Trim(href, "/"), title: nodeText(link),
			author: author(n), category: category,
		})
	}
	pager := find(doc, "div", "page")
	if len(entries) == 0 || pager == nil {
		return nil, "", fmt.Errorf("未识别完本榜条目或分页")
	}
	return entries, nextPage(pager), nil
}

func (s *source) details(ctx context.Context, id string) (book, error) {
	path := "/" + id + "/"
	doc, err := s.page(ctx, path)
	if err != nil {
		return book{}, err
	}
	header := find(doc, "div", "itemtxt")
	b := book{
		id: id, title: nodeText(find(find(header, "h1", ""), "a", "")),
		author: author(header), category: completedCategory(header),
		description: paragraphs(find(doc, "div", "des")),
		cover:       attr(find(find(doc, "div", "item"), "img", ""), "src"),
	}
	if b.category == "" {
		return book{}, fmt.Errorf("书籍 %s 未标记为已完结，或分类缺失", id)
	}
	if b.title == "" || b.author == "" || b.cover == "" || len(b.description) == 0 {
		return book{}, fmt.Errorf("书籍 %s 的书名、作者、封面或简介缺失", id)
	}
	return b, nil
}

func (s *source) catalog(ctx context.Context, b book) ([]chapter, error) {
	base := "/" + b.id + "/"
	seen := make(map[string]bool)
	var toc []chapter
	var latest []string
	for pageNo := 1; ; pageNo++ {
		path := base
		if pageNo > 1 {
			path += fmt.Sprintf("p-%d.html", pageNo)
		}
		doc, err := s.page(ctx, path)
		if err != nil {
			return nil, err
		}
		if header := find(doc, "div", "itemtxt"); pageNo == 1 && header != nil {
			header := find(doc, "div", "itemtxt")
			for n := range header.Descendants() {
				href := attr(n, "href")
				if strings.HasPrefix(href, base) && chapterPath.MatchString(href) {
					latest = append(latest, href)
				}
			}
		}
		list := byID(doc, "list")
		first := len(toc)
		if list != nil {
			for n := range list.Descendants() {
				href := attr(n, "href")
				if n.Data != "a" || !strings.HasPrefix(href, base) || !chapterPath.MatchString(href) {
					continue
				}
				title := nodeText(n)
				if seen[href] || title == "" {
					return nil, fmt.Errorf("%s 有重复或缺标题的章节: %s", path, href)
				}
				seen[href] = true
				toc = append(toc, chapter{path: href, title: title})
			}
		}
		if len(toc) == first {
			return nil, fmt.Errorf("%s 未找到章节目录", path)
		}
		next := nextPage(byID(doc, "pages"))
		if next == "" {
			break
		}
		u, err := s.base.ResolveReference(&url.URL{Path: path}).Parse(next)
		if err != nil || u.Scheme != s.base.Scheme || u.Host != s.base.Host ||
			u.Path != base+fmt.Sprintf("p-%d.html", pageNo+1) {
			return nil, fmt.Errorf("%s 的目录分页链接异常: %s", path, next)
		}
	}
	for _, path := range latest {
		if !seen[path] {
			return nil, fmt.Errorf("目录缺少详情页列出的章节: %s", path)
		}
	}
	return toc, nil
}

func (s *source) chapterBody(ctx context.Context, ch chapter) ([]string, error) {
	stem := strings.TrimSuffix(ch.path, ".html")
	path := ch.path
	var body []string
	for pageNo := 1; ; pageNo++ {
		doc, err := s.page(ctx, path)
		if err != nil {
			return nil, err
		}
		paras := paragraphs(find(doc, "div", "con"))
		pager := find(doc, "div", "prenext")
		if pageNo == 1 && len(paras) > 0 && paras[0] == ch.title {
			paras = paras[1:]
		}
		if len(paras) == 0 || pager == nil {
			return nil, fmt.Errorf("%s 未识别正文或章节导航", path)
		}
		body = append(body, paras...)
		next := nextPage(pager)
		if next == "" {
			return body, nil
		}
		path = fmt.Sprintf("%s-%d.html", stem, pageNo+1)
		if next != path {
			return nil, fmt.Errorf("章节分页链接异常: %s", next)
		}
	}
}

func (s *source) volumeBodies(ctx context.Context, b book, toc []chapter) ([][]string, error) {
	doc, err := s.page(ctx, "/"+b.id+"/txt.html")
	if err != nil {
		return nil, err
	}
	list := byID(doc, "list")
	if list == nil {
		return nil, fmt.Errorf("未找到分卷下载列表")
	}
	bodies := make([][]string, len(toc))
	var failures []error
	for n := range list.Descendants() {
		if n.Type != html.ElementNode || n.Data != "a" {
			continue
		}
		href := attr(n, "href")
		u, err := url.Parse(href)
		if err != nil || u.Path != "/txt/" || u.Query().Get("id") != b.id || u.Query().Get("p") == "" {
			failures = append(failures, fmt.Errorf("分卷下载链接异常: %s", href))
			continue
		}
		name := nodeText(n)
		m := volumeRange.FindStringSubmatch(name)
		if m == nil {
			failures = append(failures, fmt.Errorf("未识别分卷范围: %s", name))
			continue
		}
		start, _ := strconv.Atoi(m[1])
		end, _ := strconv.Atoi(m[2])
		if start < 1 || end < start || end > len(toc) {
			failures = append(failures, fmt.Errorf("分卷范围与目录不符: %s", name))
			continue
		}
		data, err := s.get(ctx, href, 64<<20)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
			continue
		}
		part, err := splitVolume(data, toc[start-1:end])
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
		}
		for i, body := range part {
			if len(body) > 0 && bodies[start-1+i] == nil {
				bodies[start-1+i] = body
			}
		}
	}
	return bodies, errors.Join(failures...)
}

type volumeChapter struct {
	title    string
	body     []string
	repeated bool
}

func titleKey(title string) string {
	return chapterdetect.TextKey(epubexport.UnescapeHTML(title))
}

// TXT 中独立的章节标题不缩进，正文行带全角缩进。
func parseVolume(data []byte) ([]volumeChapter, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("TXT 不是有效的 UTF-8")
	}
	var chapters []volumeChapter
	footer := false
	for raw := range strings.SplitSeq(strings.TrimPrefix(string(data), "\ufeff"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if footer {
			return nil, fmt.Errorf("TXT 卷尾之后仍有内容")
		}
		if strings.HasPrefix(line, "更多精彩小说，请访问：速读谷") {
			footer = true
			continue
		}
		if strings.HasPrefix(raw, "　") || strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t") {
			if len(chapters) == 0 {
				return nil, fmt.Errorf("TXT 首个章名前出现正文")
			}
			current := &chapters[len(chapters)-1]
			if len(current.body) == 0 && titleKey(line) == current.title {
				current.repeated = true
			} else {
				current.body = append(current.body, line)
			}
			continue
		}
		if len(chapters) == 0 && (strings.HasPrefix(line, "《") || strings.HasPrefix(line, "作者：") ||
			strings.HasPrefix(line, "来源：") || strings.HasPrefix(line, "网址：")) {
			continue
		}
		chapters = append(chapters, volumeChapter{title: titleKey(line)})
	}
	if !footer || len(chapters) == 0 {
		return nil, fmt.Errorf("缺少章节或卷尾")
	}
	return chapters, nil
}

func splitVolume(data []byte, toc []chapter) ([][]string, error) {
	chapters, err := parseVolume(data)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(toc))
	for i, ch := range toc {
		keys[i] = titleKey(ch.title)
	}
	matches := make([]int, len(chapters)) // 0 为未匹配，其他值为从 1 开始的目录位置。
	prefix := 0
	for prefix < min(len(chapters), len(keys)) && chapters[prefix].title == keys[prefix] {
		matches[prefix] = prefix + 1
		prefix++
	}
	for i, j := len(chapters)-1, len(keys)-1; i >= prefix && j >= prefix && chapters[i].title == keys[j]; i, j = i-1, j-1 {
		matches[i] = j + 1
	}

	bodies := make([][]string, len(toc))
	titleCount := make(map[string]int)
	if prefix != len(keys) || len(chapters) != len(keys) {
		for _, key := range keys {
			titleCount[key]++
		}
	}
	matched := 0
	for i, chapter := range chapters {
		if matches[i] == 0 || len(chapter.body) == 0 || titleCount[chapter.title] > 1 {
			continue
		}
		// 未对齐的下一行可能是意外失去缩进的正文。只有它在正文里重复了自己的章名，才把它视为章界。
		if i+1 < len(chapters) && matches[i+1] == 0 && !chapters[i+1].repeated {
			continue
		}
		bodies[matches[i]-1] = chapter.body
		matched++
	}
	if matched != len(toc) {
		return bodies, fmt.Errorf("仅对齐了 %d/%d 章", matched, len(toc))
	}
	return bodies, nil
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
	for _, name := range strings.Fields(attr(n, "class")) {
		if name == class {
			return true
		}
	}
	return false
}

func find(root *html.Node, tag, class string) *html.Node {
	if root != nil {
		for n := range root.Descendants() {
			if n.Type == html.ElementNode && n.Data == tag && (class == "" || hasClass(n, class)) {
				return n
			}
		}
	}
	return nil
}

func byID(root *html.Node, id string) *html.Node {
	for n := range root.Descendants() {
		if attr(n, "id") == id {
			return n
		}
	}
	return nil
}

func nodeText(n *html.Node) string {
	var text strings.Builder
	if n != nil {
		for child := range n.Descendants() {
			switch {
			case child.Type == html.TextNode:
				text.WriteString(child.Data)
			case child.Type == html.ElementNode && child.Data == "br":
				text.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(text.String())
}

func paragraphs(root *html.Node) []string {
	var lines []string
	if root == nil {
		return lines
	}
	for n := range root.Descendants() {
		if n.Type == html.ElementNode && n.Data == "p" {
			for line := range strings.SplitSeq(nodeText(n), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					lines = append(lines, line)
				}
			}
		}
	}
	return lines
}

func author(header *html.Node) string {
	if header != nil {
		for n := range header.Descendants() {
			if n.Type == html.ElementNode && n.Data == "a" {
				if name, ok := strings.CutPrefix(nodeText(n), "作者："); ok {
					return strings.TrimSpace(name)
				}
			}
		}
	}
	return ""
}

func completedCategory(header *html.Node) string {
	p := find(header, "p", "")
	status := find(p, "span", "")
	if nodeText(status) != "已完结" {
		return ""
	}
	for n := status.NextSibling; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode && n.Data == "span" {
			return nodeText(n)
		}
	}
	return ""
}

func nextPage(pager *html.Node) string {
	if pager != nil {
		for n := range pager.Descendants() {
			if n.Type == html.ElementNode && n.Data == "a" && nodeText(n) == "下一页" {
				return attr(n, "href")
			}
		}
	}
	return ""
}
