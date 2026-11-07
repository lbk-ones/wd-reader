package resourcecatalog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// attribute 按名称读取 HTML 节点属性，不存在时返回空字符串。
func attribute(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// elementText 递归提取节点可见文本，跳过 script、style 并合并多余空白。
func elementText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Data == "script" || n.Data == "style" {
		return ""
	}
	var b strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		b.WriteString(elementText(child))
		b.WriteByte(' ')
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// walkElements 深度优先遍历 HTML 元素节点，并对每个元素调用访问函数。
func walkElements(n *html.Node, visit func(*html.Node)) {
	if n.Type == html.ElementNode {
		visit(n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		walkElements(child, visit)
	}
}

// parseHTML 优先按 UTF-8 解析；发现非 UTF-8 字节时交给 charset 检测并转码。
func parseHTML(data []byte) (*html.Node, error) {
	if utf8.Valid(data) {
		return html.Parse(bytes.NewReader(data))
	}
	reader, err := charset.NewReader(bytes.NewReader(data), "")
	if err != nil {
		return nil, err
	}
	return html.Parse(reader)
}

// queryVariants 根据别名分组扩展书名；去重后最多返回 3 个查询变体。
func queryVariants(s Source, query string) []string {
	out := []string{query}
	seen := map[string]bool{clean(query): true}
	for name, aliases := range s.Aliases {
		group := append([]string{name}, aliases...)
		found := false
		for _, alias := range group {
			if clean(alias) == clean(query) {
				found = true
			}
		}
		if found {
			for _, alias := range group {
				if alias = strings.TrimSpace(alias); alias != "" && !seen[clean(alias)] && len(out) < 3 {
					out = append(out, alias)
					seen[clean(alias)] = true
				}
			}
			break
		}
	}
	return out
}

// searchHTML 按书名、别名及作者提示搜索同站页面，识别详情链接后补全下载文件并排序。
func searchHTML(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	pattern, err := regexp.Compile(s.BookLinkPattern)
	if err != nil {
		return SearchResponse{}, err
	}
	// 保留书名变体用于结果过滤，作者提示只补充查询词，不放宽书名匹配。
	variants := queryVariants(s, query)
	terms := append([]string{}, variants...)
	preferredAuthor := ""
	for name, author := range s.AuthorHints {
		if clean(name) == clean(query) {
			preferredAuthor = author
			if strings.TrimSpace(author) != "" {
				terms = append(terms, author)
			}
			break
		}
	}
	books := []Book{}
	seen := map[string]bool{}
	var lastError error
	successful := 0
	hasExact := func() bool {
		for _, b := range books {
			for _, q := range variants {
				if clean(b.Title) == clean(q) {
					return true
				}
			}
		}
		return false
	}
	// 每个查询词各自维护分页队列，跨页、跨别名的书籍按详情 URL 去重。
	for _, term := range terms {
		target := strings.ReplaceAll(s.SearchURL, "{query}", url.QueryEscape(term))
		queue := []string{target}
		seenPages := map[string]bool{target: true}
		maxPages := s.MaxSearchPages
		if maxPages < 1 {
			maxPages = 1
		}
		for page := 0; len(queue) > 0 && page < maxPages; page++ {
			target = queue[0]
			queue = queue[1:]
			base, err := url.Parse(target)
			if err != nil {
				return SearchResponse{}, err
			}
			data, err := fetchHTMLSearch(ctx, client, s, target, term, page == 0)
			if err != nil {
				lastError = err
				continue
			}
			doc, err := parseHTML(data)
			if err != nil {
				lastError = err
				continue
			}
			successful++
			walkElements(doc, func(n *html.Node) {
				if n.Data != "a" {
					return
				}
				ref, err := url.Parse(attribute(n, "href"))
				if err != nil {
					return
				}
				link := base.ResolveReference(ref)
				if !IsWebURL(link.String()) || !strings.EqualFold(link.Host, base.Host) {
					return
				}
				// 仅跟随同站、同路径且只改变 page 参数的分页链接。
				if link.Path == base.Path {
					params := link.Query()
					pageNumber, _ := strconv.Atoi(params.Get("page"))
					params.Del("page")
					baseParams := base.Query()
					baseParams.Del("page")
					if pageNumber > 1 && params.Encode() == baseParams.Encode() {
						link.Fragment = ""
						key := link.String()
						if !seenPages[key] {
							seenPages[key] = true
							queue = append(queue, key)
						}
					}
				}
				if !pattern.MatchString(link.Path) {
					return
				}
				title := strings.TrimSpace(attribute(n, "title"))
				if title == "" {
					title = elementText(n)
				}
				if title == "" {
					return
				}
				match := false
				for _, variant := range variants {
					if matchesTitle(title, variant) {
						match = true
						break
					}
				}
				if !match {
					return
				}
				link.Fragment = ""
				key := link.String()
				if seen[key] {
					return
				}
				seen[key] = true
				books = append(books, Book{ID: key, Title: title, URL: key})
			})
			if hasExact() {
				break
			}
			// 老作品常排在新同人之后；无精确命中时优先访问站点给出的较后分页。
			sort.SliceStable(queue, func(i, j int) bool {
				a, _ := url.Parse(queue[i])
				b, _ := url.Parse(queue[j])
				ap, _ := strconv.Atoi(a.Query().Get("page"))
				bp, _ := strconv.Atoi(b.Query().Get("page"))
				return ap > bp
			})
		}
	}
	if successful == 0 {
		return SearchResponse{Books: []Book{}}, lastError
	}
	exact := func(b Book) bool {
		for _, q := range variants {
			if clean(b.Title) == clean(q) {
				return true
			}
		}
		return false
	}
	sort.SliceStable(books, func(i, j int) bool { return exact(books[i]) && !exact(books[j]) })
	// 先读取实际文件，再为结果标记同名或相关作品，避免续作冒充原作。
	response, err := hydrateBooks(ctx, client, s, books)
	for i := range response.Books {
		response.Books[i].MatchType = "related"
		if exact(response.Books[i]) {
			response.Books[i].MatchType = "exact"
		}
	}
	if preferredAuthor != "" {
		sort.SliceStable(response.Books, func(i, j int) bool {
			return clean(response.Books[i].Author) == clean(preferredAuthor) && clean(response.Books[j].Author) != clean(preferredAuthor)
		})
	}
	return response, err
}

// fetchHTMLSearch 首个搜索页按配置提交 POST 表单，其余分页沿用站点链接执行 GET。
func fetchHTMLSearch(ctx context.Context, client *http.Client, s Source, target, query string, firstPage bool) ([]byte, error) {
	if !firstPage || !strings.EqualFold(s.SearchMethod, "POST") {
		return fetch(ctx, client, target)
	}
	// 使用表单编码处理中文、& 等字符，避免检索词被误拆成多个参数。
	form := url.Values{}
	for key, value := range s.SearchForm {
		form.Set(key, strings.ReplaceAll(value, "{query}", query))
	}
	var body io.Reader = strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return readCatalogResponse(client, req)
}

// readHTMLFiles 从详情页提取书名、作者和下载地址；无直接文件时最多访问一个匹配的同站中转页。
func readHTMLFiles(ctx context.Context, client *http.Client, s Source, b Book) ([]File, string, error) {
	data, err := fetch(ctx, client, b.URL)
	if err != nil {
		return nil, "", err
	}
	doc, err := parseHTML(data)
	if err != nil {
		return nil, "", err
	}
	pattern, err := regexp.Compile(s.DownloadLinkPattern)
	if err != nil {
		return nil, "", err
	}
	base, _ := url.Parse(b.URL)
	title := b.Title
	author := ""
	files := []File{}
	seen := map[string]bool{}
	// 正则只取第一捕获组，再用 HTML 解析清理标签和实体，而非直接显示源码。
	matchContent := func(pattern string) string {
		if pattern == "" {
			return ""
		}
		re, e := regexp.Compile(pattern)
		if e != nil {
			return ""
		}
		m := re.FindSubmatch(data)
		if len(m) < 2 {
			return ""
		}
		fragment, e := parseHTML(m[1])
		if e != nil {
			return ""
		}
		return strings.TrimSpace(elementText(fragment))
	}
	if matched := matchContent(s.TitlePattern); matched != "" {
		title = matched
	}
	if matched := matchContent(s.AuthorPattern); matched != "" {
		author = matched
	}
	walkElements(doc, func(n *html.Node) {
		if n.Data == "meta" {
			key := attribute(n, "property")
			if key == "" {
				key = attribute(n, "name")
			}
			content := strings.TrimSpace(attribute(n, "content"))
			if s.TitleMeta != "" && key == s.TitleMeta && content != "" {
				title = content
			}
			if s.AuthorMeta != "" && key == s.AuthorMeta {
				author = content
			}
		}
	})
	// 详情页和中转页共用同一收集器，按完整下载 URL 去重，最多保留 12 项。
	collectFiles := func(document *html.Node, pageURL string) {
		base, _ := url.Parse(pageURL)
		walkElements(document, func(n *html.Node) {
			if n.Data != "a" || len(files) >= 12 {
				return
			}
			ref, e := url.Parse(strings.TrimSpace(attribute(n, "href")))
			if e != nil {
				return
			}
			target := base.ResolveReference(ref)
			raw := target.String()
			if !IsWebURL(raw) || !pattern.MatchString(raw) || seen[raw] {
				return
			}
			ext := strings.ToLower(filepath.Ext(target.Path))
			format := ""
			archiveFormat := ""
			// 优先使用链接扩展名，其次处理 ZIP，最后回退到无扩展名接口的配置格式。
			if supportedExtension(ext) {
				format = strings.ToUpper(ext[1:])
			} else if ext == ".zip" && s.ZipFormat != "" {
				archiveFormat = s.ZipFormat
				format = strings.ToUpper(s.ZipFormat) + " (ZIP)"
			} else if s.DownloadFormat != "" {
				ext = "." + s.DownloadFormat
				format = strings.ToUpper(s.DownloadFormat)
			} else {
				return
			}
			name := title
			// 分卷下载使用链接中的文件名保留章节范围；只有扩展名匹配才采纳。
			if s.FileNameFromLink {
				linkName := elementText(n)
				if strings.EqualFold(filepath.Ext(linkName), ext) {
					name = linkName
				}
			}
			seen[raw] = true
			files = append(files, File{URL: raw, Name: SafeFileName(name, ext), Format: format, ArchiveFormat: archiveFormat, Referer: pageURL})
		})
	}
	collectFiles(doc, b.URL)
	// 仅在详情页未发现文件时尝试一个同站中转页，避免递归爬取整站。
	if len(files) == 0 && s.DownloadPagePattern != "" {
		pagePattern, e := regexp.Compile(s.DownloadPagePattern)
		if e != nil {
			return nil, "", e
		}
		downloadPage := ""
		walkElements(doc, func(n *html.Node) {
			if n.Data != "a" || downloadPage != "" {
				return
			}
			ref, e := url.Parse(strings.TrimSpace(attribute(n, "href")))
			if e != nil {
				return
			}
			link := base.ResolveReference(ref)
			if IsWebURL(link.String()) && strings.EqualFold(link.Host, base.Host) && pagePattern.MatchString(link.Path) {
				link.Fragment = ""
				downloadPage = link.String()
			}
		})
		if downloadPage != "" {
			pageData, e := fetch(ctx, client, downloadPage)
			if e != nil {
				return nil, "", e
			}
			pageDoc, e := parseHTML(pageData)
			if e != nil {
				return nil, "", e
			}
			collectFiles(pageDoc, downloadPage)
		}
	}
	// 验证页作为来源读取失败返回，不将其伪装成无结果或电子书文件。
	if len(files) == 0 && (bytes.Contains(data, []byte("captcha")) || bytes.Contains(data, []byte("cf-chl-"))) {
		return nil, "", fmt.Errorf("资源页面要求验证，暂时无法读取下载链接")
	}
	return files, author, nil
}
