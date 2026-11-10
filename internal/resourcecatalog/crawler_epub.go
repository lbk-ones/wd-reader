package resourcecatalog

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"wd-reader/internal/epubexport"

	"golang.org/x/net/html"
	"golang.org/x/text/language"
)

// crawlerEPUBMetadata 以详情页为准填写 EPUB 信息，书名和作者缺失时使用检索阶段的值。
func (s Source) crawlerEPUBMetadata(doc *html.Node, page *url.URL, file File) (epubexport.Book, string) {
	title, author := s.bookMetadata(doc)
	if title == "" {
		title = strings.TrimSpace(file.BookTitle)
	}
	if title == "" {
		title = strings.TrimSuffix(SafeFileName(file.Name, ".epub"), ".epub")
	}
	if author == "" {
		author = strings.TrimSpace(file.BookAuthor)
	}
	author = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(author, "作者："), "作者:"))

	var rules CrawlerBookRules
	if s.Crawler.Book != nil {
		rules = *s.Crawler.Book
	}
	meta := epubexport.Book{
		Title: title, Author: author, Source: page.String(),
		Category: firstText(doc, rules.Category, `meta[property="og:novel:category"], meta[name="og:novel:category"]`,
			`meta[property="article:section"], meta[property="og:novel:genre"]`),
	}
	meta.Category = strings.TrimSpace(strings.TrimPrefix(meta.Category, "分类："))
	if description := firstText(doc, rules.Description,
		`meta[property="og:novel:description"], meta[name="og:novel:description"]`,
		`meta[property="og:description"], meta[name="og:description"]`,
		`meta[name="description"]`, "#bookIntro, #bookintro, #intro, .book_intro"); description != "" {
		meta.Description = []string{description}
	}
	var pageLanguage string
	if roots := novelSelect(doc, "html"); len(roots) > 0 {
		pageLanguage = attribute(roots[0], "lang")
		if pageLanguage == "" {
			pageLanguage = attribute(roots[0], "xml:lang")
		}
	}
	for _, value := range []string{pageLanguage, firstText(doc, `meta[property="og:locale"], meta[name="og:locale"]`)} {
		if tag, err := language.Parse(strings.ReplaceAll(strings.TrimSpace(value), "_", "-")); err == nil && tag != language.Und {
			meta.Language = tag.String()
			break
		}
	}
	return meta, crawlerCoverURL(doc, page, rules.Cover)
}

// crawlerCoverURL 支持自定义选择器、常见封面元数据和延迟加载图片，只采用同站地址。
func crawlerCoverURL(doc *html.Node, page *url.URL, selector string) string {
	for _, rule := range []string{selector,
		`meta[property="og:novel:cover"], meta[name="og:novel:cover"]`,
		`meta[property="og:image"], meta[name="og:image"], meta[property="og:image:secure_url"]`,
		`link[rel="image_src"], #fmimg img, img#fmimg, #bookimg img, .book-cover img, .cover img, img[itemprop="image"]`,
	} {
		for _, node := range novelSelect(doc, rule) {
			for _, key := range []string{"content", "href", "data-src", "data-original", "src"} {
				if target := novelLink(page, attribute(node, key)); target != "" && target != page.String() {
					return target
				}
			}
		}
	}
	return ""
}

// cover 通过本任务的限速器读取可选封面，站点无封面或响应不可用时生成默认封面。
func (d novelDownload) cover(ctx context.Context, target, referer string) ([]byte, error) {
	if target != "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-d.tick:
			if data := fetchCrawlerCover(ctx, d.client, target, referer); len(data) > 0 {
				return data, nil
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return epubexport.DefaultCover()
}

// fetchCrawlerCover 读取大小受限的图片，按内容识别格式，避免把返回的验证页打包为封面。
func fetchCrawlerCover(ctx context.Context, client *http.Client, target, referer string) []byte {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", catalogUserAgent)
	req.Header.Set("Referer", referer)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	const coverLimit = 4 << 20
	if resp.StatusCode != http.StatusOK || resp.ContentLength > coverLimit {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, coverLimit+1))
	if err != nil || len(data) > coverLimit {
		return nil
	}
	if name, _ := epubexport.CoverFormat(data); name == "" {
		return nil
	}
	return data
}
