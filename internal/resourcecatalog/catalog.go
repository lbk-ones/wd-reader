// Package resourcecatalog 负责下载来源配置、书籍检索、下载链接解析和电子书文件下载。
package resourcecatalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// defaults 随程序嵌入默认来源，供首次创建配置及编辑器恢复默认时读取。
//
//go:embed defaults.json
var defaults []byte

// DefaultConfig 返回内置默认配置原文，仅用于生成编辑草稿，不覆盖用户文件。
func DefaultConfig() string { return string(defaults) }

// Source 定义单个下载来源的检索、元数据提取与下载规则；配置字段按来源类型选择使用。
type Source struct {
	// HTML 搜索请求方法；空值按 GET 处理，也可配置 POST。
	SearchMethod string `json:"searchMethod,omitempty"`
	// POST 表单参数模板，值中的 {query} 替换为当前检索词。
	SearchForm map[string]string `json:"searchForm,omitempty"`
	// file-index 来源的 JSON 文件目录地址。
	IndexURL string `json:"indexUrl,omitempty"`
	// 中转下载页的路径正则；详情页无直接链接时使用。
	DownloadPagePattern string `json:"downloadPagePattern,omitempty"`
	// 无文件扩展名的下载接口所返回的电子书格式。
	DownloadFormat string `json:"downloadFormat,omitempty"`
	// 是否从下载链接文字读取文件名，用于保留分卷章节范围。
	FileNameFromLink bool `json:"fileNameFromLink,omitempty"`
	// 从详情页提取书名的正则，第一捕获组为结果。
	TitlePattern string `json:"titlePattern,omitempty"`
	// 从详情页提取作者的正则，第一捕获组为结果。
	AuthorPattern string `json:"authorPattern,omitempty"`
	// 书名对应的作者提示；HTML 来源用于补充查询和排序，文件目录来源用于显示作者标签。
	AuthorHints map[string]string `json:"authorHints,omitempty"`
	// 每个检索词最多读取的搜索页数；0 按 1 页处理。
	MaxSearchPages int `json:"maxSearchPages,omitempty"`
	// HTML 搜索结果中书籍详情链接的路径正则。
	BookLinkPattern string `json:"bookLinkPattern,omitempty"`
	// 实际下载地址的完整 URL 正则，用于排除无关链接。
	DownloadLinkPattern string `json:"downloadLinkPattern,omitempty"`
	// 详情页中存放书名的 meta 标签名称或 property 值。
	TitleMeta string `json:"titleMeta,omitempty"`
	// 详情页中存放作者的 meta 标签名称或 property 值。
	AuthorMeta string `json:"authorMeta,omitempty"`
	// ZIP 内预期的电子书格式，如 txt；空值表示不接收 ZIP 链接。
	ZipFormat string `json:"zipFormat,omitempty"`
	// 书名及别名分组，用于同一本书的多名称检索。
	Aliases map[string][]string `json:"aliases,omitempty"`
	// 来源唯一标识，供前端选择、过滤和后端查找配置。
	ID string `json:"id"`
	// 来源显示名称。
	Name string `json:"name"`
	// 是否允许从该来源发起检索。
	Enabled bool `json:"enabled"`
	// 适配器类型：archive、gutenberg、html 或 file-index。
	Kind string `json:"kind"`
	// 来源说明，供前端提示资源类型和下载特点。
	Description string `json:"description"`
	// 搜索地址模板，通常使用 {query} 占位符。
	SearchURL string `json:"searchUrl"`
	// 结构化来源的详情接口模板，使用 {id} 占位符。
	DetailURL string `json:"detailUrl"`
	// 下载地址模板；按来源使用 {id}、{file} 占位符。
	DownloadURL string `json:"downloadUrl,omitempty"`
	// 书籍原站详情地址模板，供用户查看来源。
	BookURL string `json:"bookUrl"`
}

// Config 是独立配置文件的 JSON 根结构。
type Config struct {
	// 配置格式版本，目前只接受 1。
	Version int `json:"version"`
	// 来源列表，允许为空，最多配置 20 个来源。
	Sources []Source `json:"sources"`
}

// ConfigFile 将配置文件的位置、原文和解析结果一起返回，便于界面编辑或修复错误。
type ConfigFile struct {
	// 配置文件路径；配置解析失败时也保留此值。
	Path string `json:"path"`
	// 磁盘上的完整 JSON 原文。
	Content string `json:"content"`
	// 解析后的来源列表。
	Sources []Source `json:"sources"`
	// 供界面展示的配置加载错误，由桌面服务填充。
	Error string `json:"error,omitempty"`
}

// File 描述一个可下载文件，也保留重试和 ZIP 解压所需的信息。
type File struct {
	// ZIP 内要提取的电子书格式；普通文件留空。
	ArchiveFormat string `json:"archiveFormat,omitempty"`
	// 下载请求使用的来源页面地址，用于需要 Referer 的站点。
	Referer string `json:"referer,omitempty"`
	// 实际下载地址，必须为不含用户凭据的 HTTP(S) URL。
	URL string `json:"url"`
	// 建议的本地文件名，下载前还会执行文件名清理。
	Name string `json:"name"`
	// 界面显示格式，例如 TXT、EPUB 或 TXT (ZIP)。
	Format string `json:"format"`
	// 已知文件大小，单位字节；未知时为 0。
	Size int64 `json:"size"`
}

// Book 表示一个来源中的书籍及其可下载版本；同书不同来源保持独立。
type Book struct {
	// 书名匹配类型：exact 表示同名或别名，related 表示相关作品；部分适配器不设置。
	MatchType string `json:"matchType,omitempty"`
	// 来源内的书籍标识，可以是资源编号、详情 URL 或目录文件路径。
	ID string `json:"id"`
	// 书名；HTML 下载文件名可另从详情页提取。
	Title string `json:"title"`
	// 作者名称，可能来自详情元数据或配置中的作者提示。
	Author string `json:"author"`
	// 原站书籍详情地址。
	URL string `json:"url"`
	// 所属来源标识，用于结果过滤。
	SourceID string `json:"sourceId"`
	// 所属来源显示名称。
	SourceName string `json:"sourceName"`
	// 已发现的下载文件；无可下载文件的书籍不会进入最终结果。
	Files []File `json:"files"`
}

// SearchResponse 区分可下载结果、无文件条目及读取失败条目，允许部分来源详情失败。
type SearchResponse struct {
	// 包含实际下载文件的书籍列表。
	Books []Book `json:"books"`
	// 详情读取成功但无可用文件的候选数量。
	Skipped int `json:"skipped"`
	// 详情读取或等待并发槽位失败的候选数量。
	Failed int `json:"failed"`
}

// IsWebURL 校验 HTTP(S) 地址，要求存在主机且不携带用户名或密码。
func IsWebURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

// Parse 解析并校验配置版本、来源标识、地址模板、正则和分页范围；保存和加载共用这套规则。
func Parse(data []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Version != 1 || c.Sources == nil || len(c.Sources) > 20 {
		return c, fmt.Errorf("version must be 1; sources must be an array of at most 20 sources")
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		if s.ID == "" || strings.TrimSpace(s.Name) == "" || seen[s.ID] {
			return c, fmt.Errorf("source id/name missing or duplicate: %s", s.ID)
		}
		seen[s.ID] = true
		if s.Kind != "archive" && s.Kind != "gutenberg" && s.Kind != "html" && s.Kind != "file-index" {
			return c, fmt.Errorf("%s: unsupported resource source kind", s.ID)
		}
		// 文件目录来源直接从索引构造详情和下载地址，不要求通用搜索接口模板。
		if s.Kind == "file-index" {
			if !IsWebURL(s.IndexURL) {
				return c, fmt.Errorf("%s: invalid indexUrl", s.ID)
			}
			for field, raw := range map[string]string{"downloadUrl": s.DownloadURL, "bookUrl": s.BookURL} {
				if !strings.Contains(raw, "{file}") || !IsWebURL(strings.ReplaceAll(raw, "{file}", "test.txt")) {
					return c, fmt.Errorf("%s: invalid %s", s.ID, field)
				}
			}
			continue
		}
		// 先检查请求方式和查询占位符，避免保存后才发现配置无法发起搜索。
		method := strings.ToUpper(s.SearchMethod)
		if method != "" && method != "GET" && method != "POST" {
			return c, fmt.Errorf("%s: searchMethod must be GET or POST", s.ID)
		}
		if method == "POST" && s.Kind != "html" {
			return c, fmt.Errorf("%s: POST is supported for HTML sources", s.ID)
		}
		urls := map[string]string{"searchUrl": s.SearchURL}
		if s.Kind != "html" {
			urls["detailUrl"] = s.DetailURL
			urls["bookUrl"] = s.BookURL
		}
		for field, raw := range urls {
			if field == "searchUrl" && method == "POST" {
				hasQuery := strings.Contains(raw, "{query}")
				for _, v := range s.SearchForm {
					hasQuery = hasQuery || strings.Contains(v, "{query}")
				}
				if !hasQuery || !IsWebURL(strings.ReplaceAll(raw, "{query}", "test")) {
					return c, fmt.Errorf("%s: POST search needs an HTTP(S) URL and {query} in its URL or form", s.ID)
				}
				continue
			}
			placeholder := "{id}"
			if field == "searchUrl" {
				placeholder = "{query}"
			}
			if !strings.Contains(raw, placeholder) || !IsWebURL(strings.ReplaceAll(raw, placeholder, "test")) {
				return c, fmt.Errorf("%s: invalid %s", s.ID, field)
			}
		}
		// HTML 规则在保存阶段编译校验，下载扩展名和分页数量也在此约束。
		if s.Kind == "html" {
			for field, pattern := range map[string]string{"downloadPagePattern": s.DownloadPagePattern, "titlePattern": s.TitlePattern, "authorPattern": s.AuthorPattern} {
				if pattern != "" {
					if _, err := regexp.Compile(pattern); err != nil {
						return c, fmt.Errorf("%s: invalid %s: %w", s.ID, field, err)
					}
				}
			}
			if s.DownloadFormat != "" && !supportedExtension("."+s.DownloadFormat) {
				return c, fmt.Errorf("%s: invalid downloadFormat", s.ID)
			}
			if s.MaxSearchPages < 0 || s.MaxSearchPages > 10 {
				return c, fmt.Errorf("%s: maxSearchPages must be 0–10", s.ID)
			}
			for field, pattern := range map[string]string{"bookLinkPattern": s.BookLinkPattern, "downloadLinkPattern": s.DownloadLinkPattern} {
				if pattern == "" {
					return c, fmt.Errorf("%s: %s is required", s.ID, field)
				}
				if _, err := regexp.Compile(pattern); err != nil {
					return c, fmt.Errorf("%s: invalid %s: %w", s.ID, field, err)
				}
			}
			if s.ZipFormat != "" && !supportedExtension("."+s.ZipFormat) {
				return c, fmt.Errorf("%s: zipFormat must be txt, epub, mobi, azw or azw3", s.ID)
			}
		}
		if s.Kind == "archive" && (!strings.Contains(s.DownloadURL, "{id}") || !strings.Contains(s.DownloadURL, "{file}") || !IsWebURL(strings.NewReplacer("{id}", "test", "{file}", "test").Replace(s.DownloadURL))) {
			return c, fmt.Errorf("%s: invalid downloadUrl", s.ID)
		}
	}
	return c, nil
}

// Load 读取独立配置；文件不存在时以排他方式创建默认文件，已有配置即使无效也保留原文。
func Load(path string) (ConfigFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return ConfigFile{}, err
		}
		// 排他创建：若其他窗口同时初始化成功，直接读取其文件，避免覆盖。
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if e == nil {
			_, e = f.Write(defaults)
			closeErr := f.Close()
			if e == nil {
				e = closeErr
			}
		}
		if e != nil && !os.IsExist(e) {
			return ConfigFile{}, e
		}
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return ConfigFile{}, err
	}
	c, err := Parse(data)
	return ConfigFile{Path: path, Content: string(data), Sources: c.Sources}, err
}

// Save 先校验配置，再写入同目录临时文件并同步、重命名，避免无效内容或写入中断破坏原配置。
func Save(path, content string) error {
	if len(content) > 1024*1024 {
		return fmt.Errorf("config exceeds 1 MiB")
	}
	if _, err := Parse([]byte(content)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// 临时文件与目标在同一目录，写完并同步后才替换正式配置。
	f, err := os.CreateTemp(filepath.Dir(path), ".resource-sources-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// fetch 构造可随上下文取消的 GET 请求，再交给统一响应读取逻辑。
func fetch(ctx context.Context, client *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return readCatalogResponse(client, req)
}

// readCatalogResponse 统一设置请求标识、检查 HTTP 状态，并将目录或 HTML 响应限制在 4 MiB 内。
func readCatalogResponse(client *http.Client, req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36 Edg/157.0.0.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 多读一个字节来区分刚好达到上限与真正超限，不能只依赖响应头大小。
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("catalog exceeds 4 MiB")
	}
	return data, nil
}

// clean 只保留 Unicode 字母和数字并转成小写，用于忽略书名中的标点和大小写差异。
func clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
}

// matchesTitle 按空白拆分查询词，要求每个非空标准化词都出现在书名中。
func matchesTitle(title, query string) bool {
	title = clean(title)
	for _, part := range strings.Fields(query) {
		if part = clean(part); part != "" && !strings.Contains(title, part) {
			return false
		}
	}
	return true
}

// scalar 兼容 Archive 的字符串及字符串数组字段，将多个值拼接为显示文本。
func scalar(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case []any:
		var a []string
		for _, x := range v {
			if s, ok := x.(string); ok {
				a = append(a, s)
			}
		}
		return strings.Join(a, " / ")
	}
	return ""
}

// truthy 兼容远端元数据中的布尔值及字符串 true、1。
func truthy(v any) bool { return v == true || v == "true" || v == "1" }

// sourceURL 将资源编号进行路径转义后替换模板中的 {id}。
func sourceURL(template, id string) string {
	return strings.ReplaceAll(template, "{id}", url.PathEscape(id))
}

// gutenbergID 从 OPDS 条目地址中提取数字书号。
var gutenbergID = regexp.MustCompile(`/ebooks/(\d+)(?:\.opds)?$`)

// reservedFilename 匹配 Windows 设备保留名，确保文件可跨平台落盘。
var reservedFilename = regexp.MustCompile(`^(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])(?:\.|$)`)

// atomFeed 对应 Gutenberg 使用的 Atom/OPDS 订阅文档。
type atomFeed struct {
	// 限定 XML 根节点为 feed。
	XMLName xml.Name `xml:"feed"`
	// 订阅中的书籍条目。
	Entries []atomEntry `xml:"entry"`
}

// atomEntry 保存 OPDS 条目的书名、作者和可获取资源链接。
type atomEntry struct {
	// 条目地址，从中提取 Gutenberg 书籍编号。
	ID string `xml:"id"`
	// 条目书名。
	Title string `xml:"title"`
	// 作者节点列表。
	Authors []struct {
		// 作者显示名称。
		Name string `xml:"name"`
	} `xml:"author"`
	// 资源与导航链接列表。
	Links []struct {
		// 链接关系，仅 acquisition 和 open-access 关系可作为下载候选。
		Rel string `xml:"rel,attr"`
		// 链接声明的 MIME 类型，用于判断电子书格式。
		Type string `xml:"type,attr"`
		// 链接地址，允许相对路径。
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

// Search 校验书名并设置 20 秒总超时，按来源类型分发检索，再补全可下载文件。
func Search(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	query = strings.TrimSpace(query)
	result := SearchResponse{Books: []Book{}}
	if clean(query) == "" || len([]rune(query)) > 100 {
		return result, fmt.Errorf("请输入 1–100 个字符的书名")
	}
	// 一个来源的搜索、分页和详情读取共用总超时，避免请求层层延长等待。
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if s.Kind == "file-index" {
		return searchFileIndex(ctx, client, s, query)
	}
	if s.Kind == "html" {
		return searchHTML(ctx, client, s, query)
	}
	// Archive 使用查询表达式，将书名作为带引号的字符串并转义特殊字符。
	term := query
	if s.Kind == "archive" {
		term = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(query) + `"`
	}
	data, err := fetch(ctx, client, strings.ReplaceAll(s.SearchURL, "{query}", url.QueryEscape(term)))
	if err != nil {
		return result, err
	}
	var candidates []Book
	if s.Kind == "archive" {
		var response struct {
			// Archive 搜索响应主体；指针用于区分缺失字段和空结果。
			Response *struct {
				// 搜索接口返回的候选资源列表。
				Docs []struct {
					// Archive 资源编号，用于请求元数据及拼接原站地址。
					ID string `json:"identifier"`
					// 远端书名，兼容字符串及字符串数组。
					Title any `json:"title"`
					// 远端作者，兼容字符串及字符串数组。
					Creator any `json:"creator"`
				} `json:"docs"`
			} `json:"response"`
		}
		if err = json.Unmarshal(data, &response); err != nil {
			return result, err
		}
		if response.Response == nil {
			return result, fmt.Errorf("资源目录未返回有效的查询结果")
		}
		for _, item := range response.Response.Docs {
			title := scalar(item.Title)
			if item.ID != "" && matchesTitle(title, query) {
				candidates = append(candidates, Book{ID: item.ID, Title: title, Author: scalar(item.Creator)})
			}
		}
	} else {
		var feed atomFeed
		if err = xml.Unmarshal(data, &feed); err != nil {
			return result, err
		}
		for _, entry := range feed.Entries {
			m := gutenbergID.FindStringSubmatch(entry.ID)
			if len(m) > 1 && matchesTitle(entry.Title, query) {
				candidates = append(candidates, Book{ID: m[1], Title: entry.Title})
			}
		}
	}
	return hydrateBooks(ctx, client, s, candidates)
}

// hydrateBooks 最多读取 8 本候选、同时处理 4 本详情；保留输入顺序并剔除无文件条目。
func hydrateBooks(ctx context.Context, client *http.Client, s Source, candidates []Book) (SearchResponse, error) {
	result := SearchResponse{Books: []Book{}}
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	// 详情请求允许部分失败；固定并发上限，避免一次搜索同时压满站点连接。
	slots := make(chan struct{}, 4)
	var mu sync.Mutex
	var wg sync.WaitGroup
	// 每个协程写回原下标，网络完成顺序不会改变候选结果的排序。
	books := make([]Book, len(candidates))
	for i, candidate := range candidates {
		wg.Add(1)
		go func(i int, b Book) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				mu.Lock()
				result.Failed++
				mu.Unlock()
				return
			}
			b.SourceID = s.ID
			b.SourceName = s.Name
			if s.Kind != "html" {
				b.URL = sourceURL(s.BookURL, b.ID)
			}
			files, author, e := readFiles(ctx, client, s, b)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				result.Failed++
				return
			}
			// 目录命中不等于存在下载文件；无文件与请求失败分别计数。
			if len(files) == 0 {
				result.Skipped++
				return
			}
			b.Files = files
			if b.Author == "" {
				b.Author = author
			}
			books[i] = b
		}(i, candidate)
	}
	wg.Wait()
	for _, b := range books {
		if len(b.Files) > 0 {
			result.Books = append(result.Books, b)
		}
	}
	// 有成功结果时返回部分结果；全部不可用且存在读取失败时提示重试。
	if len(result.Books) == 0 && result.Failed > 0 {
		return result, fmt.Errorf("%d 个资源的文件列表读取失败，请重试", result.Failed)
	}
	return result, nil
}

// readFiles 读取单本书的详情，按来源筛选公开可下载文件并返回作者信息。
func readFiles(ctx context.Context, client *http.Client, s Source, b Book) ([]File, string, error) {
	if s.Kind == "html" {
		return readHTMLFiles(ctx, client, s, b)
	}
	data, err := fetch(ctx, client, sourceURL(s.DetailURL, b.ID))
	if err != nil {
		return nil, "", err
	}
	files := []File{}
	author := ""
	seen := map[string]bool{}
	if s.Kind == "archive" {
		var meta struct {
			// 资源级元数据，包含作者和访问限制等信息。
			Metadata map[string]any `json:"metadata"`
			// 元数据中的实际文件清单，不等同于全部文件都可下载。
			Files []struct {
				// 远端文件名，可能含目录或不安全字符，落盘前需清理。
				Name string `json:"name"`
				// 远端文件大小，Archive 使用字符串表示字节数。
				Size string `json:"size"`
				// 远端格式说明，也用于识别加密和 LCP 文件。
				Format string `json:"format"`
				// 文件是否私有，兼容布尔值及字符串标记。
				Private any `json:"private"`
			} `json:"files"`
			// 资源是否已隐藏；隐藏资源不展示下载项。
			IsDark bool `json:"is_dark"`
			// 接口返回的资源不可用错误。
			Error any `json:"error"`
		}
		if err = json.Unmarshal(data, &meta); err != nil {
			return nil, "", err
		}
		if meta.Error != nil {
			return nil, "", fmt.Errorf("resource unavailable")
		}
		// Archive 的不可见或受限条目不作为可公开下载资源。
		if meta.IsDark || truthy(meta.Metadata["access-restricted-item"]) {
			return files, "", nil
		}
		author = scalar(meta.Metadata["creator"])
		// 过滤私有、加密和 LCP 文件，再按实际下载地址去重。
		for _, file := range meta.Files {
			ext := strings.ToLower(filepath.Ext(file.Name))
			format := strings.ToLower(file.Format)
			if !supportedExtension(ext) || truthy(file.Private) || strings.Contains(format, "encrypted") || strings.Contains(format, "lcp") || strings.HasSuffix(strings.ToLower(file.Name), "_lcp.epub") {
				continue
			}
			target := strings.ReplaceAll(sourceURL(s.DownloadURL, b.ID), "{file}", url.PathEscape(file.Name))
			if !IsWebURL(target) || seen[target] {
				continue
			}
			seen[target] = true
			size, _ := strconv.ParseInt(file.Size, 10, 64)
			files = append(files, File{URL: target, Name: SafeFileName(file.Name, ext), Format: strings.ToUpper(ext[1:]), Size: size})
			if len(files) == 12 {
				break
			}
		}
	} else {
		var feed atomFeed
		if err = xml.Unmarshal(data, &feed); err != nil {
			return nil, "", err
		}
		for _, entry := range feed.Entries {
			if author == "" {
				var names []string
				for _, a := range entry.Authors {
					names = append(names, a.Name)
				}
				author = strings.Join(names, " / ")
			}
			// OPDS 只接受可获取资源关系；购买等导航链接不会成为下载项。
			for _, link := range entry.Links {
				if link.Rel != "http://opds-spec.org/acquisition" && link.Rel != "http://opds-spec.org/acquisition/open-access" {
					continue
				}
				ext := ""
				switch strings.Split(link.Type, ";")[0] {
				case "application/epub+zip":
					ext = ".epub"
				case "text/plain":
					ext = ".txt"
				case "application/x-mobipocket-ebook":
					ext = ".mobi"
					if strings.Contains(link.Href, ".kf8.") {
						ext = ".azw3"
					}
				}
				if ext == "" || seen[ext] {
					continue
				}
				base, _ := url.Parse(s.DetailURL)
				ref, e := url.Parse(link.Href)
				if e != nil {
					continue
				}
				target := base.ResolveReference(ref).String()
				if !IsWebURL(target) {
					continue
				}
				seen[ext] = true
				files = append(files, File{URL: target, Name: SafeFileName(b.Title, ext), Format: strings.ToUpper(ext[1:])})
			}
		}
	}
	return files, author, nil
}

// supportedExtension 限定可直接导入的电子书扩展名；传入值应已转换成小写。
func supportedExtension(ext string) bool {
	return ext == ".txt" || ext == ".epub" || ext == ".mobi" || ext == ".azw" || ext == ".azw3"
}

// SafeFileName 移除路径和非法字符，限制文件名长度，避开 Windows 保留名后补齐扩展名。
func SafeFileName(name, ext string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if strings.EqualFold(filepath.Ext(name), ext) {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	runes := []rune(name)
	if len(runes) > 100 {
		name = string(runes[:100])
	}
	// 中文字符可能占多个 UTF-8 字节；字符数限制后再控制字节长度。
	for len(name) > 220 {
		runes = []rune(name)
		name = string(runes[:len(runes)-1])
	}
	if name == "" {
		name = "book"
	}
	if reservedFilename.MatchString(strings.ToUpper(name)) {
		name = "book-" + name
	}
	return name + ext
}
