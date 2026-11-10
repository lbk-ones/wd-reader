package resourcecatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// CrawlerRules 保存 web-crawler 专用的页面解析规则。请求地址、表单和分组等通用配置在 Source 中定义。
type CrawlerRules struct {
	// MinInterval 设置同一下载任务的请求节奏，单位为毫秒。运行时共享的限速定时器至少间隔 300 毫秒。
	MinInterval int `json:"minInterval,omitempty"`
	// Concurrency 是每批并行抓取的章节数。零值按 1 处理，运行时最多并行 4 章。
	Concurrency int `json:"concurrency,omitempty"`
	// Search 描述书名搜索结果的页面结构。填写 Source.SearchURL 时必须提供。
	Search *CrawlerSearchRules `json:"search,omitempty"`
	// Book 可覆盖详情页书名和作者的默认提取位置。
	Book *CrawlerBookRules `json:"book,omitempty"`
	// TOC 描述如何找到完整目录、识别章节链接并遍历目录分页。
	TOC CrawlerTOCRules `json:"toc"`
	// Chapter 描述如何提取正文、清理内容并合并同一章的多个页面。
	Chapter CrawlerChapterRules `json:"chapter"`
}

// CrawlerSearchRules 描述搜索结果页的选择器和站点特有的请求参数。
type CrawlerSearchRules struct {
	// Result 匹配每条结果的容器，相对于整张搜索结果页执行。
	Result string `json:"result"`
	// BookName 在单条结果内匹配书名链接，链接文字和 href 分别提供书名和详情地址。
	BookName string `json:"bookName"`
	// Author 在单条结果内匹配作者文字，可留空。
	Author string `json:"author,omitempty"`
	// Cookies 是搜索请求附带的 Cookie 请求头原文。
	Cookies string `json:"cookies,omitempty"`
	// Next 匹配后续结果页的链接。留空时只读取首次响应。
	Next string `json:"next,omitempty"`
	// JSONP 启用全本小说网的查询参数编码，用于其返回 JSONP 的搜索接口。
	JSONP bool `json:"jsonp,omitempty"`
	// TitlePrefix 控制是否去掉书名前的方括号标签，便于判断是否与检索词同名。
	TitlePrefix bool `json:"titlePrefix,omitempty"`
}

// CrawlerBookRules 定义详情页的书名、作者、简介、分类和封面选择器。
// 书名和作者优先采用 Source 中显式配置的 Meta 字段。
type CrawlerBookRules struct {
	// BookName 匹配书名。若命中 meta 元素，读取 content 属性。
	BookName string `json:"bookName,omitempty"`
	// Author 匹配作者。若命中 meta 元素，读取 content 属性。
	Author string `json:"author,omitempty"`
	// Description 匹配书籍简介，留空时尝试通用元数据和常见简介容器。
	Description string `json:"description,omitempty"`
	// Category 匹配书籍分类，留空时尝试站点元数据。
	Category string `json:"category,omitempty"`
	// Cover 匹配封面图片或封面 meta 元素，地址须在书源站点内。
	Cover string `json:"cover,omitempty"`
}

// CrawlerTOCRules 描述目录入口、分页链接和章节条目。
type CrawlerTOCRules struct {
	// URL 是独立目录页的地址模板。留空时直接从详情页读取目录。
	// {id} 取自 Source.BookLinkPattern 的第一个捕获组。
	URL string `json:"url,omitempty"`
	// BaseURI 覆盖相对章节链接的解析基准。留空时使用当前目录页的实际响应地址。
	BaseURI string `json:"baseUri,omitempty"`
	// Item 匹配章节链接。节点文字作为章节标题，href 作为正文地址。
	Item string `json:"item"`
	// NextPage 匹配目录分页链接或下拉选项。普通链接使用 href，下拉选项可使用 value。
	NextPage string `json:"nextPage,omitempty"`
	// HiddenList 匹配包含隐藏干扰条目的列表，抓取前按页面的样式规则清理。
	HiddenList string `json:"hiddenList,omitempty"`
	// OnclickPath 用于还原脚本式章节链接。{chapter} 来自 read_tz 的数字参数，{id} 来自书籍路径。
	OnclickPath string `json:"onclickPath,omitempty"`
	// IsDesc 表示单页章节列表倒序排列，抓取时会将该页的条目反转。
	IsDesc bool `json:"isDesc,omitempty"`
}

// CrawlerChapterRules 描述单章页面的正文位置、清理方式和章内翻页方式。
type CrawlerChapterRules struct {
	// Title 匹配正文页的章名。比较各页章名可辅助判断是否已进入下一章。
	Title string `json:"title,omitempty"`
	// Content 匹配正文容器，下载时只提取这些容器中的内容。
	Content string `json:"content"`
	// FilterTag 匹配正文容器内要移除的节点，例如广告或导航栏。
	FilterTag string `json:"filterTag,omitempty"`
	// FilterTxt 是纯文本清理使用的 Go 正则表达式，命中的文字会被删除。
	FilterTxt string `json:"filterTxt,omitempty"`
	// NextPage 匹配章内翻页链接。最终还需根据链接和章节目录判断是否应继续合并。
	NextPage string `json:"nextPage,omitempty"`
	// ScriptNext 指定从脚本文本中提取翻页地址的方式，可选 nextpage 或 keyboard。
	ScriptNext string `json:"scriptNext,omitempty"`
	// Base64 表示正文藏在指定脚本调用的 Base64 字符串中，需解码后再提取文本。
	Base64 bool `json:"base64,omitempty"`
}

// validateCrawler 在读取或保存配置时校验抓取规则，提前发现缺失字段、无效选择器和越界设置。
func (s Source) validateCrawler() error {
	rules := s.Crawler
	if !IsWebURL(s.HomeURL) || rules == nil {
		return fmt.Errorf("web-crawler needs homeUrl and crawler rules")
	}
	if s.MaxSearchPages < 0 || s.MaxSearchPages > 10 || rules.Concurrency < 0 || rules.Concurrency > 5 ||
		rules.MinInterval < 0 || rules.MinInterval > 10000 {
		return fmt.Errorf("web-crawler limits: maxSearchPages 0–10, concurrency 0–5, minInterval 0–10000 ms")
	}
	if rules.TOC.Item == "" || rules.Chapter.Content == "" {
		return fmt.Errorf("crawler.toc.item and crawler.chapter.content are required")
	}
	pattern, err := regexp.Compile(s.BookLinkPattern)
	if err != nil {
		return fmt.Errorf("invalid bookLinkPattern: %w", err)
	}
	// 地址模板依赖书籍编号时，路径正则必须能提供对应的捕获组。
	for _, target := range []string{rules.TOC.URL, rules.TOC.BaseURI, rules.TOC.OnclickPath} {
		if strings.Contains(target, "{id}") && pattern.NumSubexp() == 0 {
			return fmt.Errorf("crawler.toc uses {id}; bookLinkPattern needs a capturing group")
		}
	}
	home, _ := url.Parse(s.HomeURL)
	for _, target := range []string{rules.TOC.URL, rules.TOC.BaseURI} {
		if target != "" && novelLink(home, strings.ReplaceAll(target, "{id}", "test")) == "" {
			return fmt.Errorf("crawler.toc.url and baseUri must point to the source site")
		}
	}
	if _, err := regexp.Compile(rules.Chapter.FilterTxt); err != nil {
		return fmt.Errorf("invalid crawler.chapter.filterTxt: %w", err)
	}
	switch rules.Chapter.ScriptNext {
	case "", "nextpage", "keyboard":
	default:
		return fmt.Errorf("crawler.chapter.scriptNext must be nextpage or keyboard")
	}
	selectors := []string{rules.TOC.Item, rules.TOC.NextPage, rules.TOC.HiddenList,
		rules.Chapter.Title, rules.Chapter.Content, rules.Chapter.FilterTag, rules.Chapter.NextPage}
	if rules.Book != nil {
		selectors = append(selectors, rules.Book.BookName, rules.Book.Author, rules.Book.Description, rules.Book.Category, rules.Book.Cover)
	}
	// 搜索地址留空代表仅通过详情链接使用该书源，此时目录和正文规则仍需有效。
	if s.SearchURL != "" {
		search := rules.Search
		if search == nil || search.Result == "" || search.BookName == "" {
			return fmt.Errorf("crawler.search.result and crawler.search.bookName are required for title search")
		}
		if err := s.validateSearchRequest(search.JSONP); err != nil {
			return err
		}
		selectors = append(selectors, search.Result, search.BookName, search.Author, search.Next)
	}
	for _, selector := range selectors {
		if selector != "" {
			if _, err := cascadia.Compile(selector); err != nil {
				return fmt.Errorf("invalid CSS selector %q: %w", selector, err)
			}
		}
	}
	return nil
}

// sameNovelHost 比较主机和端口。主机名忽略大小写，并将带 www. 和不带 www. 的域名视为同站。
func sameNovelHost(a, b *url.URL) bool {
	host := func(u *url.URL) string { return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") }
	return host(a) == host(b) && a.Port() == b.Port()
}

// bookMatch 检查地址是否属于当前书源的非首页路径，并返回路径正则的匹配结果供目录模板取编号。
func (r Source) bookMatch(raw string) ([]string, bool) {
	if !IsWebURL(raw) || !IsWebURL(r.HomeURL) {
		return nil, false
	}
	target, _ := url.Parse(raw)
	home, _ := url.Parse(r.HomeURL)
	if !sameNovelHost(target, home) || strings.Trim(target.Path, "/") == "" {
		return nil, false
	}
	if r.BookLinkPattern == "" {
		return nil, true
	}
	pattern, err := regexp.Compile(r.BookLinkPattern)
	if err != nil {
		return nil, false
	}
	// 保留 URL 的路径转义，便于把捕获到的编号放回地址模板。
	match := pattern.FindStringSubmatch(target.EscapedPath())
	return match, match != nil
}

// isBookURL 用于只需要判断详情地址是否匹配、无需读取书籍编号的调用场景。
func (r Source) isBookURL(raw string) bool {
	_, ok := r.bookMatch(raw)
	return ok
}

// bookResult 将抓取结果封装成统一的书籍条目。下载项携带详情地址和书源标识，供队列启动逐章抓取。
func (r Source) bookResult(target, title, author string) Book {
	return Book{
		ID: target, Title: title, Author: author, URL: target, SourceID: r.ID, SourceName: r.Name,
		Files: []File{{
			NovelSourceID: r.ID, URL: target, Name: SafeFileName(title, ".epub"), Format: "EPUB",
			BookTitle: title, BookAuthor: author, SourceGroup: r.Group,
		}},
	}
}

// ResolveNovelBook 按配置顺序匹配已启用的抓取书源，读取用户粘贴的详情页并返回可下载条目。
func ResolveNovelBook(ctx context.Context, client *http.Client, sources []Source, raw string) (Book, error) {
	raw = strings.TrimSpace(raw)
	for _, rule := range sources {
		if !rule.Enabled || rule.Kind != CrawlerKind || rule.Crawler == nil || !rule.isBookURL(raw) {
			continue
		}
		u, _ := url.Parse(raw)
		u.Fragment = ""
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		doc, target, err := fetchNovelPage(ctx, newNovelClient(client), http.MethodGet, u.String(), nil, "")
		if err != nil {
			return Book{}, err
		}
		if !rule.isBookURL(target.String()) {
			return Book{}, fmt.Errorf("详情页已跳转到不支持的地址")
		}
		title, author := rule.bookMetadata(doc)
		// 未配置路径规则时，再检查页面是否包含章节目录，以免把站内其他页面误认成书籍。
		if title == "" || (rule.BookLinkPattern == "" && len(novelSelect(doc, rule.Crawler.TOC.Item)) == 0) {
			return Book{}, fmt.Errorf("无法识别书籍，请粘贴小说详情页链接")
		}
		return rule.bookResult(target.String(), title, author), nil
	}
	return Book{}, fmt.Errorf("该链接没有匹配的已启用网页小说书源")
}

// newNovelClient 复用基础客户端的网络设置，为本次操作创建独立的 Cookie 会话并约束重定向。
func newNovelClient(base *http.Client) *http.Client {
	client := *base
	client.Jar, _ = cookiejar.New(nil)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !IsWebURL(req.URL.String()) || !sameNovelHost(req.URL, via[0].URL) {
			return fmt.Errorf("站点重定向到其他地址或次数过多")
		}
		if base.CheckRedirect != nil {
			return base.CheckRedirect(req, via)
		}
		return nil
	}
	return &client
}

// fetchNovelPage 发起有超时和响应大小限制的页面请求，返回解析后的 HTML 及最终响应地址。
// 最终地址供调用方解析相对链接，以适配站点重定向。
func fetchNovelPage(ctx context.Context, client *http.Client, method, target string, form url.Values, cookie string) (*html.Node, *url.URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("User-Agent", catalogUserAgent)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if method == http.MethodPost {
		req.Header.Set("Referer", req.URL.Scheme+"://"+req.URL.Host+"/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.Header.Get("Cf-Mitigated") == "challenge" || resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("访问书源失败 (HTTP %d)", resp.StatusCode)
	}
	const maxPageSize = 4 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > maxPageSize {
		return nil, nil, fmt.Errorf("书源页面超过 4 MiB")
	}
	if isChallengePage(data) {
		return nil, nil, fmt.Errorf("书源要求浏览器验证")
	}
	// JSONP 搜索响应的 content 字段包含结果页 HTML，直接解析数据即可提取结果。
	if payload := bytes.TrimSpace(data); bytes.HasPrefix(payload, []byte("search(")) {
		var result struct{ Content string }
		if end := bytes.LastIndexByte(payload, ')'); end < len("search(") ||
			json.Unmarshal(payload[len("search("):end], &result) != nil {
			return nil, nil, fmt.Errorf("书源搜索响应无法解析")
		}
		data = []byte(result.Content)
	}
	// 优先保留有效的 UTF-8 内容，其余页面根据响应头和 HTML 编码信息尝试转码。
	var reader io.Reader = bytes.NewReader(data)
	if !utf8.Valid(data) {
		reader, err = charset.NewReader(reader, resp.Header.Get("Content-Type"))
		if err != nil {
			return nil, nil, err
		}
	}
	doc, err := html.Parse(reader)
	return doc, resp.Request.URL, err
}

// novelSelect 在指定节点的子树中执行 CSS 选择器。空选择器代表该项规则未启用。
func novelSelect(doc *html.Node, selector string) []*html.Node {
	if selector == "" || doc == nil {
		return nil
	}
	matcher, err := cascadia.Compile(selector)
	if err != nil {
		return nil
	}
	return cascadia.QueryAll(doc, matcher)
}

// novelText 读取选择器命中的文本。普通元素取可见文字，meta 元素取 content 属性。
func novelText(doc *html.Node, selector string) string {
	for _, node := range novelSelect(doc, selector) {
		if value := attribute(node, "content"); node.Data == "meta" {
			return strings.TrimSpace(value)
		}
		if value := elementText(node); value != "" {
			return value
		}
	}
	return ""
}

// firstText 依次尝试多个选择器，返回首个非空值。
func firstText(doc *html.Node, selectors ...string) string {
	for _, selector := range selectors {
		if value := novelText(doc, selector); value != "" {
			return value
		}
	}
	return ""
}

// bookMetadata 提取详情页书名和作者。依次采用配置的 Meta 字段、自定义选择器和常见页面位置。
func (r Source) bookMetadata(doc *html.Node) (string, string) {
	var title, author string
	for _, meta := range novelSelect(doc, "meta") {
		for _, key := range []string{attribute(meta, "property"), attribute(meta, "name")} {
			if key != "" && key == r.TitleMeta {
				title = attribute(meta, "content")
			}
			if key != "" && key == r.AuthorMeta {
				author = attribute(meta, "content")
			}
		}
	}
	var selectors CrawlerBookRules
	if r.Crawler.Book != nil {
		selectors = *r.Crawler.Book
	}
	if title == "" {
		title = firstText(doc, selectors.BookName, `meta[property="og:novel:book_name"], meta[name="og:novel:book_name"]`,
			"#info h1", "h1", `meta[property="og:title"], meta[name="og:title"]`, "title")
	}
	if author == "" {
		author = firstText(doc, selectors.Author, `meta[property="og:novel:author"], meta[name="og:novel:author"]`,
			`meta[name="author"], meta[property="book:author"]`, "a[rel=author]", "#info p")
	}
	return title, strings.TrimSpace(strings.TrimPrefix(author, "作者："))
}

// novelLink 以当前页面地址解析相对链接，保留同站 HTTP(S) 地址并去掉用于页面定位的片段。
func novelLink(base *url.URL, ref string) string {
	u, err := base.Parse(strings.TrimSpace(ref))
	if strings.TrimSpace(ref) == "" || err != nil || !IsWebURL(u.String()) || !sameNovelHost(base, u) {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

// novelTitlePrefix 匹配搜索结果书名前的方括号标签。
var novelTitlePrefix = regexp.MustCompile(`^\[[^\]]*\]\s*`)

// searchBooks 用书名查询单个抓取书源，按配置读取分页并把同名结果排在前面。
func (r Source) searchBooks(ctx context.Context, client *http.Client, query string) (SearchResponse, error) {
	result := SearchResponse{Books: []Book{}}
	if r.Crawler == nil || r.Crawler.Search == nil || r.SearchURL == "" {
		return result, fmt.Errorf("此书源仅支持小说详情页链接")
	}
	rule := r.Crawler.Search
	client = newNovelClient(client)
	target := strings.ReplaceAll(r.SearchURL, "{query}", url.QueryEscape(query))
	if rule.JSONP {
		target = novelJSONPSearch(target, query)
	}
	var form url.Values
	method := http.MethodGet
	if strings.EqualFold(r.SearchMethod, http.MethodPost) {
		method, form = http.MethodPost, url.Values{}
		for key, value := range r.SearchForm {
			form.Set(key, strings.ReplaceAll(value, "{query}", query))
		}
	}
	maxPages := r.MaxSearchPages
	if maxPages == 0 {
		maxPages = 3
	}
	// 分别记录已请求的页面和已收录的详情链接，避免循环翻页和跨页重复结果。
	seenPages, seenBooks := map[string]bool{}, map[string]bool{}
	for page := 0; page < maxPages && target != "" && !seenPages[target]; page++ {
		seenPages[target] = true
		doc, base, err := fetchNovelPage(ctx, client, method, target, form, rule.Cookies)
		if err != nil {
			if page == 0 {
				return result, err
			}
			// 首页成功时保留已取得的结果，将后续分页错误交给前端单独提示。
			result.Failed++
			break
		}
		for _, row := range novelSelect(doc, rule.Result) {
			for _, link := range novelSelect(row, rule.BookName) {
				u, err := base.Parse(attribute(link, "href"))
				title := elementText(link)
				if rule.TitlePrefix {
					title = novelTitlePrefix.ReplaceAllString(title, "")
				}
				if err != nil || attribute(link, "href") == "" || title == "" || !r.isBookURL(u.String()) {
					continue
				}
				u.Fragment = ""
				key := u.String()
				if !seenBooks[key] {
					seenBooks[key] = true
					book := r.bookResult(key, title, firstText(row, rule.Author))
					if clean(title) == clean(query) {
						book.MatchType = "exact"
					} else {
						book.MatchType = "related"
					}
					result.Books = append(result.Books, book)
				}
			}
		}
		// 有些站点在只命中一本书时直接返回详情页，用书名再次校验后收录。
		if page == 0 && len(result.Books) == 0 && r.isBookURL(base.String()) &&
			(base.String() != target || len(novelSelect(doc, r.Crawler.TOC.Item)) > 0) {
			title, author := r.bookMetadata(doc)
			if title != "" && matchesTitle(title, query) {
				book := r.bookResult(base.String(), title, author)
				if clean(title) == clean(query) {
					book.MatchType = "exact"
				}
				result.Books = append(result.Books, book)
			}
		}
		// 后续页按站点提供的同站链接发起 GET 请求，初次搜索的表单无需再次提交。
		target, method, form = "", http.MethodGet, nil
		for _, link := range novelSelect(doc, rule.Next) {
			href := novelLink(base, attribute(link, "href"))
			if href != base.String() && !seenPages[href] && href != "" {
				target = href
				break
			}
		}
	}
	sort.SliceStable(result.Books, func(i, j int) bool {
		return result.Books[i].MatchType == "exact" && result.Books[j].MatchType != "exact"
	})
	result.Books = result.Books[:min(len(result.Books), 20)]
	return result, nil
}

// novelJSONPSearch 生成全本小说网要求的 JSONP 请求参数。
// b 参数由 URL 编码后的关键词按站点字符表转换，并在每个字符两侧加入随机字符组成。
func novelJSONPSearch(target, query string) string {
	const alphabet = "PXhw7UT1B0a9kQDKZsjIASmOezxYG4CHo5Jyfg2b8FLpEvRr3WtVnlqMidu6cN"
	encoded := strings.ReplaceAll(url.QueryEscape(query), "+", "%20")
	var token strings.Builder
	for _, char := range []byte(encoded) {
		if index := strings.IndexByte(alphabet, char); index >= 0 {
			char = alphabet[(index+3)%len(alphabet)]
		}
		token.WriteByte(alphabet[rand.IntN(len(alphabet))])
		token.WriteByte(char)
		token.WriteByte(alphabet[rand.IntN(len(alphabet))])
	}
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	params := u.Query()
	params.Set("c", "book")
	params.Set("a", "search.json")
	params.Set("callback", "search")
	params.Set("keywords", query)
	params.Set("b", token.String())
	u.RawQuery = params.Encode()
	return u.String()
}
