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

func attribute(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
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
func walkElements(n *html.Node, visit func(*html.Node)) {
	if n.Type == html.ElementNode {
		visit(n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		walkElements(child, visit)
	}
}
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
func searchHTML(ctx context.Context, client *http.Client, s Source, query string) (SearchResponse, error) {
	pattern, err := regexp.Compile(s.BookLinkPattern)
	if err != nil {
		return SearchResponse{}, err
	}
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
			// Older originals often appear after recent fan fiction. Follow the site's last-page link first.
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

func fetchHTMLSearch(ctx context.Context, client *http.Client, s Source, target, query string, firstPage bool) ([]byte, error) {
	if !firstPage || !strings.EqualFold(s.SearchMethod, "POST") {
		return fetch(ctx, client, target)
	}
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
	if len(files) == 0 && (bytes.Contains(data, []byte("captcha")) || bytes.Contains(data, []byte("cf-chl-"))) {
		return nil, "", fmt.Errorf("资源页面要求验证，暂时无法读取下载链接")
	}
	return files, author, nil
}
