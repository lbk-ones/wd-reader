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

//go:embed defaults.json
var defaults []byte

type Source struct {
	SearchMethod        string              `json:"searchMethod,omitempty"`
	SearchForm          map[string]string   `json:"searchForm,omitempty"`
	IndexURL            string              `json:"indexUrl,omitempty"`
	DownloadPagePattern string              `json:"downloadPagePattern,omitempty"`
	DownloadFormat      string              `json:"downloadFormat,omitempty"`
	FileNameFromLink    bool                `json:"fileNameFromLink,omitempty"`
	TitlePattern        string              `json:"titlePattern,omitempty"`
	AuthorPattern       string              `json:"authorPattern,omitempty"`
	AuthorHints         map[string]string   `json:"authorHints,omitempty"`
	MaxSearchPages      int                 `json:"maxSearchPages,omitempty"`
	BookLinkPattern     string              `json:"bookLinkPattern,omitempty"`
	DownloadLinkPattern string              `json:"downloadLinkPattern,omitempty"`
	TitleMeta           string              `json:"titleMeta,omitempty"`
	AuthorMeta          string              `json:"authorMeta,omitempty"`
	ZipFormat           string              `json:"zipFormat,omitempty"`
	Aliases             map[string][]string `json:"aliases,omitempty"`
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Enabled             bool                `json:"enabled"`
	Kind                string              `json:"kind"`
	Description         string              `json:"description"`
	SearchURL           string              `json:"searchUrl"`
	DetailURL           string              `json:"detailUrl"`
	DownloadURL         string              `json:"downloadUrl,omitempty"`
	BookURL             string              `json:"bookUrl"`
}
type Config struct {
	Version int      `json:"version"`
	Sources []Source `json:"sources"`
}
type ConfigFile struct {
	Path    string   `json:"path"`
	Content string   `json:"content"`
	Sources []Source `json:"sources"`
	Error   string   `json:"error,omitempty"`
}
type File struct {
	ArchiveFormat string `json:"archiveFormat,omitempty"`
	Referer       string `json:"referer,omitempty"`
	URL           string `json:"url"`
	Name          string `json:"name"`
	Format        string `json:"format"`
	Size          int64  `json:"size"`
}
type Book struct {
	MatchType  string `json:"matchType,omitempty"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	URL        string `json:"url"`
	SourceID   string `json:"sourceId"`
	SourceName string `json:"sourceName"`
	Files      []File `json:"files"`
}
type SearchResponse struct {
	Books   []Book `json:"books"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
}

func IsWebURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}
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
				return c, fmt.Errorf("%s: zipFormat must be txt, epub, mobi or azw3", s.ID)
			}
		}
		if s.Kind == "archive" && (!strings.Contains(s.DownloadURL, "{id}") || !strings.Contains(s.DownloadURL, "{file}") || !IsWebURL(strings.NewReplacer("{id}", "test", "{file}", "test").Replace(s.DownloadURL))) {
			return c, fmt.Errorf("%s: invalid downloadUrl", s.ID)
		}
	}
	return c, nil
}
func Load(path string) (ConfigFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return ConfigFile{}, err
		}
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
func fetch(ctx context.Context, client *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return readCatalogResponse(client, req)
}

func readCatalogResponse(client *http.Client, req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", "wd-reader/1.0 (ebook downloads; https://github.com/lbk-ones/wd-reader)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("catalog exceeds 4 MiB")
	}
	return data, nil
}
func clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
}
func matchesTitle(title, query string) bool {
	title = clean(title)
	for _, part := range strings.Fields(query) {
		if part = clean(part); part != "" && !strings.Contains(title, part) {
			return false
		}
	}
	return true
}
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
func truthy(v any) bool { return v == true || v == "true" || v == "1" }
func sourceURL(template, id string) string {
	return strings.ReplaceAll(template, "{id}", url.PathEscape(id))
}

var gutenbergID = regexp.MustCompile(`/ebooks/(\d+)(?:\.opds)?$`)
var reservedFilename = regexp.MustCompile(`^(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])(?:\.|$)`)

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Authors []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Links []struct {
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

func Search(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	query = strings.TrimSpace(query)
	result := SearchResponse{Books: []Book{}}
	if clean(query) == "" || len([]rune(query)) > 100 {
		return result, fmt.Errorf("请输入 1–100 个字符的书名")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if s.Kind == "file-index" {
		return searchFileIndex(ctx, client, s, query)
	}
	if s.Kind == "html" {
		return searchHTML(ctx, client, s, query)
	}
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
			Response *struct {
				Docs []struct {
					ID      string `json:"identifier"`
					Title   any    `json:"title"`
					Creator any    `json:"creator"`
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

func hydrateBooks(ctx context.Context, client *http.Client, s Source, candidates []Book) (SearchResponse, error) {
	result := SearchResponse{Books: []Book{}}
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	slots := make(chan struct{}, 4)
	var mu sync.Mutex
	var wg sync.WaitGroup
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
	if len(result.Books) == 0 && result.Failed > 0 {
		return result, fmt.Errorf("%d 个资源的文件列表读取失败，请重试", result.Failed)
	}
	return result, nil
}

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
			Metadata map[string]any `json:"metadata"`
			Files    []struct {
				Name    string `json:"name"`
				Size    string `json:"size"`
				Format  string `json:"format"`
				Private any    `json:"private"`
			} `json:"files"`
			IsDark bool `json:"is_dark"`
			Error  any  `json:"error"`
		}
		if err = json.Unmarshal(data, &meta); err != nil {
			return nil, "", err
		}
		if meta.Error != nil {
			return nil, "", fmt.Errorf("resource unavailable")
		}
		if meta.IsDark || truthy(meta.Metadata["access-restricted-item"]) {
			return files, "", nil
		}
		author = scalar(meta.Metadata["creator"])
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

func supportedExtension(ext string) bool {
	return ext == ".txt" || ext == ".epub" || ext == ".mobi" || ext == ".azw3"
}
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
