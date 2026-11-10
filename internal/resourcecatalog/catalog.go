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

// catalogUserAgent 统一目录查询、书源检测和下载请求使用的客户端标识。
const catalogUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36 Edg/157.0.0.0"

// SourceKind 标记书源适配器类型。字符串值用于 JSON 配置和前端绑定。
type SourceKind string

const (
	// ArchiveKind 通过 Internet Archive 的接口查询和获取成品文件。
	ArchiveKind SourceKind = "archive"
	// GutenbergKind 通过 Project Gutenberg 的 OPDS 目录获取成品文件。
	GutenbergKind SourceKind = "gutenberg"
	// HTMLKind 从网页中提取成品文件的下载链接。
	HTMLKind SourceKind = "html"
	// FileIndexKind 从文件索引中查找成品文件。
	FileIndexKind SourceKind = "file-index"
	// CrawlerKind 从网页目录逐章抓取正文并生成 EPUB。
	CrawlerKind SourceKind = "web-crawler"
)

const (
	// defaultSourceGroup 是成品电子书来源的默认分组标识，界面显示为「完本下载」。
	defaultSourceGroup = "download"
	// configVersion 表示已合并成品下载和网页抓取书源的配置版本。
	configVersion = 2
	// maxResourceSources 限制单份配置中的书源数量。
	maxResourceSources = 100
)

// configMu 串行化进程内的配置读取、迁移和写入，避免迁移期间覆盖另一处窗口刚保存的配置。
var configMu sync.Mutex

// DefaultConfig 返回内置默认配置原文，仅用于生成编辑草稿，不覆盖用户文件。
func DefaultConfig() string { return string(defaults) }

// Source 定义单个下载来源的检索、元数据提取与下载规则；配置字段按来源类型选择使用。
type Source struct {
	// SearchMethod 是搜索请求方法，空值按 GET 处理。html 和 web-crawler 也支持 POST。
	SearchMethod string `json:"searchMethod,omitempty"`
	// SearchForm 保存 POST 表单参数模板，值中的 {query} 替换为当前检索词。
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
	// MaxSearchPages 限制每个检索词读取的搜索页数。零值在 html 中按 1 页处理，在 web-crawler 中按 3 页处理。
	MaxSearchPages int `json:"maxSearchPages,omitempty"`
	// BookLinkPattern 匹配书籍详情链接的路径。web-crawler 可用第一个捕获组为目录模板提供 {id}。
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
	// Enabled 决定是否允许检索和检测该来源。关闭 web-crawler 来源也会停用其详情链接识别和新下载任务。
	Enabled bool `json:"enabled"`
	// Kind 决定使用哪个适配器解析配置、检索资源和下载。
	Kind SourceKind `json:"kind"`
	// 来源说明，供前端提示资源类型和下载特点。
	Description string `json:"description"`
	// SearchURL 是搜索地址模板，通常使用 {query} 占位符。web-crawler 留空时只接受详情链接。
	SearchURL string `json:"searchUrl,omitempty"`
	// 结构化来源的详情接口模板，使用 {id} 占位符。
	DetailURL string `json:"detailUrl,omitempty"`
	// 下载地址模板；按来源使用 {id}、{file} 占位符。
	DownloadURL string `json:"downloadUrl,omitempty"`
	// 书籍原站详情地址模板，供用户查看来源。
	BookURL string `json:"bookUrl,omitempty"`
	// Group 用于下载页分组。成品来源默认分到 download，抓取来源默认分到 main。
	Group string `json:"group"`
	// HomeURL 是书源主页，可作为访问检测入口。对 web-crawler 来说也是校验详情链接归属的基准。
	HomeURL string `json:"homeUrl,omitempty"`
	// Crawler 保存逐章下载所需的 CSS 选择器、分页和限速规则，仅供 web-crawler 使用。
	Crawler *CrawlerRules `json:"crawler,omitempty"`
}

// Config 是统一书源配置的 JSON 根结构，同时保存成品下载和逐章抓取来源。
type Config struct {
	// 配置格式版本。加载和保存时将旧版本升级为 2。
	Version int `json:"version"`
	// 来源列表，允许为空，最多配置 100 个来源。
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
	// Concurrency 是本次章节抓取的协程数。零值沿用书源设置，1 表示串行。
	Concurrency int `json:"concurrency,omitempty"`
	// SourceGroup 保存下载项所属分组，便于任务列表和重试沿用相同的显示与操作。
	SourceGroup string `json:"sourceGroup,omitempty"`
	// NovelSourceID 指向逐章抓取规则。此时 URL 是书籍详情地址，任务创建时会按该标识重新读取配置。
	NovelSourceID string `json:"novelSourceId,omitempty"`
	// BookTitle 保存检索时取得的书名，供详情页缺少书名时补充 EPUB 元数据。
	BookTitle string `json:"bookTitle,omitempty"`
	// BookAuthor 保存检索时取得的作者，供详情页缺少作者时补充 EPUB 元数据。
	BookAuthor string `json:"bookAuthor,omitempty"`
	// ZIP 内要提取的电子书格式；普通文件留空。
	ArchiveFormat string `json:"archiveFormat,omitempty"`
	// 下载请求使用的来源页面地址，用于需要 Referer 的站点。
	Referer string `json:"referer,omitempty"`
	// URL 是文件直链或书籍详情地址，必须是不含用户凭据的 HTTP(S) URL。
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
	// Books 保存带下载项的书籍，下载项可以是成品文件，也可以是待生成的 EPUB。
	Books []Book `json:"books"`
	// 详情读取成功但无可用文件的候选数量。
	Skipped int `json:"skipped"`
	// Failed 统计未能读取的候选详情或后续搜索页，具体单位由书源类型决定。
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
	if (c.Version != 1 && c.Version != configVersion) || c.Sources == nil || len(c.Sources) > maxResourceSources {
		return c, fmt.Errorf("version must be 1 or %d; sources must be an array of at most %d sources", configVersion, maxResourceSources)
	}
	seen := map[string]bool{}
	for i := range c.Sources {
		s := &c.Sources[i]
		if s.ID == "" || strings.TrimSpace(s.Name) == "" || seen[s.ID] {
			return c, fmt.Errorf("source id/name missing or duplicate: %s", s.ID)
		}
		seen[s.ID] = true
		// 统一整理分组名称，让缺少 group 字段的旧配置也能直接展示。
		s.Group = strings.TrimSpace(s.Group)
		if s.Group == "完本下载" {
			s.Group = defaultSourceGroup
		}
		if s.Group == "" {
			s.Group = defaultSourceGroup
			if s.Kind == CrawlerKind {
				s.Group = "main"
			}
		}
		if s.HomeURL != "" && !IsWebURL(s.HomeURL) {
			return c, fmt.Errorf("%s: invalid homeUrl", s.ID)
		}
		switch s.Kind {
		case CrawlerKind:
			if err := s.validateCrawler(); err != nil {
				return c, fmt.Errorf("%s: %w", s.ID, err)
			}
			continue
		case ArchiveKind, GutenbergKind, HTMLKind, FileIndexKind:
		default:
			return c, fmt.Errorf("%s: unsupported resource source kind", s.ID)
		}
		// 文件目录来源直接从索引构造详情和下载地址，不要求通用搜索接口模板。
		if s.Kind == FileIndexKind {
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
		if err := s.validateSearchRequest(false); err != nil {
			return c, fmt.Errorf("%s: %w", s.ID, err)
		}
		if s.Kind != HTMLKind {
			for field, raw := range map[string]string{"detailUrl": s.DetailURL, "bookUrl": s.BookURL} {
				if !strings.Contains(raw, "{id}") || !IsWebURL(strings.ReplaceAll(raw, "{id}", "test")) {
					return c, fmt.Errorf("%s: invalid %s", s.ID, field)
				}
			}
		}
		// HTML 规则在保存阶段编译校验，下载扩展名和分页数量也在此约束。
		if s.Kind == HTMLKind {
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
		if s.Kind == ArchiveKind && (!strings.Contains(s.DownloadURL, "{id}") || !strings.Contains(s.DownloadURL, "{file}") || !IsWebURL(strings.NewReplacer("{id}", "test", "{file}", "test").Replace(s.DownloadURL))) {
			return c, fmt.Errorf("%s: invalid downloadUrl", s.ID)
		}
	}
	return c, nil
}

// validateSearchRequest 检查请求方式、地址和检索词占位符。
// queryOptional 用于自行构造查询参数的站点，仍须提供合法的 HTTP(S) 地址。
func (s Source) validateSearchRequest(queryOptional bool) error {
	method := strings.ToUpper(s.SearchMethod)
	if method != "" && method != http.MethodGet && method != http.MethodPost {
		return fmt.Errorf("searchMethod must be GET or POST")
	}
	hasQuery := strings.Contains(s.SearchURL, "{query}")
	if method == http.MethodPost {
		if s.Kind != HTMLKind && s.Kind != CrawlerKind {
			return fmt.Errorf("POST is supported for HTML and web-crawler sources")
		}
		for _, value := range s.SearchForm {
			hasQuery = hasQuery || strings.Contains(value, "{query}")
		}
	}
	if (!hasQuery && !queryOptional) || !IsWebURL(strings.ReplaceAll(s.SearchURL, "{query}", "test")) {
		return fmt.Errorf("searchUrl needs an HTTP(S) URL and {query} in its URL or POST form")
	}
	return nil
}

// Load 读取统一书源配置，缺失时创建默认文件，旧版配置通过校验后升级并写回。
// 解析或迁移失败时返回已读取的路径和原文，供配置窗口修复。
func Load(path string) (ConfigFile, error) {
	configMu.Lock()
	defer configMu.Unlock()
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
	file := ConfigFile{Path: path, Content: string(data)}
	c, upgraded, err := prepareConfig(data)
	if err != nil {
		return file, err
	}
	if upgraded != nil {
		if err := writeConfig(path, upgraded); err != nil {
			return file, err
		}
		file.Content = string(upgraded)
	}
	file.Sources = c.Sources
	return file, nil
}

// Save 校验配置大小和规则，必要时先迁移旧格式，再用同目录临时文件替换原配置。
func Save(path, content string) error {
	if len(content) > 1024*1024 {
		return fmt.Errorf("config exceeds 1 MiB")
	}
	data := []byte(content)
	_, upgraded, err := prepareConfig(data)
	if err != nil {
		return err
	}
	if upgraded != nil {
		data = upgraded
	}
	configMu.Lock()
	defer configMu.Unlock()
	return writeConfig(path, data)
}

// prepareConfig 校验配置并计算升级结果。只有旧版配置会返回要写回的 JSON，当前版本直接沿用原文。
func prepareConfig(data []byte) (Config, []byte, error) {
	config, err := Parse(data)
	if err != nil || config.Version == configVersion {
		return config, nil, err
	}
	// 用原始 JSON 字段进行迁移，以保留结构体未声明的扩展字段和原有来源设置。
	var root map[string]json.RawMessage
	var sources []map[string]json.RawMessage
	_ = json.Unmarshal(data, &root)
	_ = json.Unmarshal(root["sources"], &sources)
	var builtIn struct {
		Sources []map[string]json.RawMessage `json:"sources"`
	}
	if err := json.Unmarshal(defaults, &builtIn); err != nil {
		return config, nil, err
	}
	ids := make(map[string]bool, len(sources))
	for i, source := range config.Sources {
		ids[source.ID] = true
		group, _ := json.Marshal(source.Group)
		sources[i]["group"] = group
	}
	// 只补入缺少的内置抓取书源。已有标识优先保留用户条目，升级后也不会恢复被删除的书源。
	for _, source := range builtIn.Sources {
		var kind SourceKind
		var id string
		_ = json.Unmarshal(source["kind"], &kind)
		_ = json.Unmarshal(source["id"], &id)
		if kind == CrawlerKind && !ids[id] {
			sources = append(sources, source)
		}
	}
	root["version"], _ = json.Marshal(configVersion)
	root["sources"], err = json.Marshal(sources)
	if err != nil {
		return config, nil, err
	}
	upgraded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return config, nil, err
	}
	upgraded = append(upgraded, '\n')
	config, err = Parse(upgraded)
	return config, upgraded, err
}

// writeConfig 在目标目录中写入、同步并关闭临时文件，最后通过重命名更新正式文件。
// 调用方负责校验内容并持有 configMu。
func writeConfig(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// 临时文件与目标在同一目录，写完并同步后才替换正式配置。
	f, err := os.CreateTemp(filepath.Dir(path), ".resource-sources-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
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
	req.Header.Set("User-Agent", catalogUserAgent)
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

// Search 校验书名并设置 20 秒总超时，按来源类型查询成品文件或生成逐章抓取的下载项。
func Search(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	query = strings.TrimSpace(query)
	result := SearchResponse{Books: []Book{}}
	if clean(query) == "" || len([]rune(query)) > 100 {
		return result, fmt.Errorf("请输入 1–100 个字符的书名")
	}
	// 一个来源的搜索、分页和详情读取共用总超时，避免请求层层延长等待。
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	switch s.Kind {
	case FileIndexKind:
		return searchFileIndex(ctx, client, s, query)
	case HTMLKind:
		return searchHTML(ctx, client, s, query)
	case CrawlerKind:
		return s.searchBooks(ctx, client, query)
	}
	// Archive 使用查询表达式，将书名作为带引号的字符串并转义特殊字符。
	term := query
	if s.Kind == ArchiveKind {
		term = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(query) + `"`
	}
	data, err := fetch(ctx, client, strings.ReplaceAll(s.SearchURL, "{query}", url.QueryEscape(term)))
	if err != nil {
		return result, err
	}
	var candidates []Book
	if s.Kind == ArchiveKind {
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
			if s.Kind != HTMLKind {
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
	if s.Kind == HTMLKind {
		return readHTMLFiles(ctx, client, s, b)
	}
	data, err := fetch(ctx, client, sourceURL(s.DetailURL, b.ID))
	if err != nil {
		return nil, "", err
	}
	files := []File{}
	author := ""
	seen := map[string]bool{}
	if s.Kind == ArchiveKind {
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
