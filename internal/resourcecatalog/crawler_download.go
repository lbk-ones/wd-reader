package resourcecatalog

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"wd-reader/internal/epubexport"

	"golang.org/x/net/html"
)

// novelChapter 是从目录提取的一个章节。章节按目录顺序保存，正文由后续请求获取。
type novelChapter struct {
	// title 用于 EPUB 目录和章节标题，target 是规范化后的同站正文地址。
	title, target string
}

// maxNovelChapters 限制一次下载处理的目录条目数，防止异常目录持续扩张。
const maxNovelChapters = 20000

// maxCrawlerConcurrency 限制单任务同时抓取的章节数，控制网络连接和正文缓冲大小。
const maxCrawlerConcurrency = 16

// novelDownload 封装一个逐章下载任务共享的书源配置、请求会话和限速器。
type novelDownload struct {
	// source 是任务启动时读取的书源快照，包含目录与正文解析规则。
	source Source
	// client 保存本任务的 Cookie 会话，并限制跨站重定向。
	client *http.Client
	// tick 由本任务的所有页面请求共享，用来限制请求发起频率。
	tick <-chan time.Time
}

// page 等待限速器放行后读取一个页面。等待期间和网络请求期间均可取消。
func (d novelDownload) page(ctx context.Context, target string) (*html.Node, *url.URL, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-d.tick:
		return fetchNovelPage(ctx, d.client, http.MethodGet, target, nil, "")
	}
}

// chapters 从详情页或独立目录页读取完整章节表，处理分页、倒序、隐藏条目和重复链接。
func (d novelDownload) chapters(ctx context.Context, bookURL string, detail *html.Node, detailURL *url.URL) ([]novelChapter, error) {
	toc := d.source.Crawler.TOC
	match, _ := d.source.bookMatch(bookURL)
	var bookID string
	if len(match) > 1 {
		bookID = match[1]
	}
	start, doc, base := detailURL.String(), detail, detailURL
	if toc.URL != "" {
		if strings.Contains(toc.URL, "{id}") && bookID == "" {
			return nil, fmt.Errorf("书籍链接缺少目录所需的编号")
		}
		start = novelLink(detailURL, strings.ReplaceAll(toc.URL, "{id}", bookID))
		if start == "" {
			return nil, fmt.Errorf("目录链接不属于当前书源")
		}
		if start != detailURL.String() {
			// 独立目录页需要另行请求。目录与详情同页时复用已经取得的 HTML。
			doc = nil
		}
	}
	if doc == nil {
		var err error
		doc, base, err = d.page(ctx, start)
		if err != nil {
			return nil, fmt.Errorf("读取目录失败: %w", err)
		}
	}
	start = base.String()
	pages := []string{start}
	firstOptions := d.catalogLinks(doc, base)
	// 如果分页选项包含当前页，采用选项给出的顺序，避免访问起点位于目录中间时打乱章节。
	if slices.Contains(firstOptions, start) {
		pages = firstOptions
	}
	queued := make(map[string]bool, len(pages))
	for _, target := range pages {
		queued[target] = true
	}
	// queued 防止目录链接来回跳转，seen 则合并多个目录页中重复出现的章节。
	seen := map[string]bool{}
	var chapters []novelChapter
	for i := 0; i < len(pages); i++ {
		if len(pages) > 500 {
			return nil, fmt.Errorf("目录超过 500 页")
		}
		page, pageURL := doc, base
		if pages[i] != start {
			var err error
			page, pageURL, err = d.page(ctx, pages[i])
			if err != nil {
				return nil, fmt.Errorf("读取目录第 %d 页失败: %w", i+1, err)
			}
		}
		for _, next := range d.catalogLinks(page, pageURL) {
			if !queued[next] {
				queued[next] = true
				pages = append(pages, next)
			}
		}
		removeHiddenNovelLinks(page, toc.HiddenList, toc.OnclickPath != "")
		links := novelSelect(page, toc.Item)
		if toc.IsDesc {
			slices.Reverse(links)
		}
		linkBase := pageURL
		if toc.BaseURI != "" {
			target := novelLink(pageURL, strings.ReplaceAll(toc.BaseURI, "{id}", bookID))
			if target == "" {
				return nil, fmt.Errorf("章节链接的基地址不属于当前书源")
			}
			linkBase, _ = url.Parse(target)
		}
		valid := false
		for _, link := range links {
			target := novelLink(linkBase, attribute(link, "href"))
			// 部分站点把章节编号放进 onclick，仅提取数字并套用配置中的地址模板。
			if (target == "" || target == linkBase.String()) && toc.OnclickPath != "" {
				if m := novelChapterOnclick.FindStringSubmatch(attribute(link, "onclick")); len(m) > 1 {
					ref := strings.NewReplacer("{id}", bookID, "{chapter}", m[1]).Replace(toc.OnclickPath)
					target = novelLink(linkBase, ref)
				}
			}
			title := elementText(link)
			if target == "" || title == "" || target == pageURL.String() || target == linkBase.String() {
				continue
			}
			// 一页里即使只有重复章节，也表明该页已正确解析。
			valid = true
			if seen[target] {
				continue
			}
			seen[target] = true
			chapters = append(chapters, novelChapter{title, target})
			if len(chapters) > maxNovelChapters {
				return nil, fmt.Errorf("章节数量超过 %d", maxNovelChapters)
			}
		}
		if !valid {
			return nil, fmt.Errorf("目录第 %d 页没有可识别的章节", i+1)
		}
	}
	if len(chapters) == 0 {
		return nil, fmt.Errorf("目录中没有可下载的章节")
	}
	return chapters, nil
}

// catalogLinks 按页面顺序收集目录分页地址，兼容普通链接和下拉选项，并去掉同页重复项。
func (d novelDownload) catalogLinks(doc *html.Node, base *url.URL) []string {
	var targets []string
	seen := map[string]bool{}
	for _, node := range novelSelect(doc, d.source.Crawler.TOC.NextPage) {
		ref := attribute(node, "href")
		if ref == "" {
			ref = attribute(node, "value")
		}
		if next := novelLink(base, ref); next != "" && !seen[next] {
			seen[next] = true
			targets = append(targets, next)
		}
	}
	return targets
}

var (
	// 从站点的章节点击处理函数中取出章节编号。
	novelChapterOnclick = regexp.MustCompile(`read_tz\(\s*(\d+)\s*\)`)
	// 匹配目录列表中从前或从后计数的隐藏位置。
	hiddenNovelItem = regexp.MustCompile(`(?i)li:nth-(child|last-child)\(\s*(\d+)\s*\)\s*\{\s*display\s*:\s*none\b`)
	// 捕获已知脚本调用中承载正文的 Base64 字符串。
	novelBase64Text = regexp.MustCompile(`(?:writeln\(\s*|qsbs\.bb\(\s*)['"]([a-zA-Z0-9+/=]+)['"]`)
	// 去除地址和标题中的页码标记后，可比较两页是否属于同一章。
	novelPageSuffix  = regexp.MustCompile(`(?i)[_-]\d+(\.[a-z]+)$`)
	novelPageHeading = regexp.MustCompile(`[（(](?:第\s*)?\d+(?:\s*/\s*\d+|\s*页)[）)]\s*$`)
	// 从常见脚本赋值和右方向键事件中定位下一页地址或保存该地址的变量。
	scriptNextPage     = regexp.MustCompile(`\bnextpage\s*=\s*["']([^"']+)["']`)
	scriptNextVariable = regexp.MustCompile(`key\s*==\s*39\)\s*location\s*=\s*([A-Za-z_$][\w$]*)`)
)

// removeHiddenNovelLinks 读取 nth-child 样式给出的位置，清理目录首尾的隐藏条目。
// 处理脚本式章节链接时只保留最长的目录列表，以排除另列的最新章节等重复内容。
func removeHiddenNovelLinks(doc *html.Node, selector string, largestOnly bool) {
	if selector == "" {
		return
	}
	var before, after int
	for _, style := range novelSelect(doc, "style") {
		for child := style.FirstChild; child != nil; child = child.NextSibling {
			for _, match := range hiddenNovelItem.FindAllStringSubmatch(child.Data, -1) {
				var position int
				fmt.Sscan(match[2], &position)
				if match[1] == "child" {
					before = max(before, position)
				} else {
					after = max(after, position)
				}
			}
		}
	}
	lists := novelSelect(doc, selector)
	var largest *html.Node
	largestCount := 0
	for _, list := range lists {
		var items []*html.Node
		for item := list.FirstChild; item != nil; item = item.NextSibling {
			if item.Type == html.ElementNode && item.Data == "li" {
				items = append(items, item)
			}
		}
		if len(items) > largestCount {
			largest, largestCount = list, len(items)
		}
		for i, item := range items {
			// 列表长度足够时才按样式裁剪，避免把较短的独立列表误删为空。
			if len(items) > before+after && (i < before || i >= len(items)-after) {
				list.RemoveChild(item)
			}
		}
	}
	if largestOnly && largest != nil {
		for _, list := range lists {
			if list != largest && list.Parent != nil {
				list.Parent.RemoveChild(list)
			}
		}
	}
}

// chapterText 合并同一章节的各页正文。directory 用来识别属于其他章节的入口地址。
// 任一正文页读取失败时返回错误，避免把缺页内容写入生成的电子书。
func (d novelDownload) chapterText(ctx context.Context, chapter novelChapter, directory map[string]bool, filter *regexp.Regexp) (string, error) {
	target := chapter.target
	seen := map[string]bool{target: true}
	var text strings.Builder
	var heading string
	for i := 0; i < 40; i++ {
		doc, base, err := d.page(ctx, target)
		if err != nil {
			return "", err
		}
		pageHeading := clean(novelPageHeading.ReplaceAllString(novelText(doc, d.source.Crawler.Chapter.Title), ""))
		// 有些站点将「下一章」混在翻页控件中。去掉页码后，章名变化即结束当前章。
		if i == 0 {
			heading = pageHeading
		} else if heading != "" && pageHeading != "" && heading != pageHeading {
			return text.String(), nil
		}
		content := d.contentText(doc, filter)
		if content == "" || strings.Contains(content, "访问太频繁了，奴家受不了啦") {
			return "", fmt.Errorf("章节正文为空或站点限制了访问频率")
		}
		if text.Len()+len(content) > 8*1024*1024 {
			return "", fmt.Errorf("章节正文超过 8 MiB")
		}
		if text.Len() > 0 {
			text.WriteString("\n\n")
		}
		text.WriteString(content)
		target = d.nextChapterPage(doc, base)
		// 目录已知入口交给对应章节处理，访问过的地址则直接结束以防循环。
		if target == "" || seen[target] || directory[target] {
			return text.String(), nil
		}
		seen[target] = true
	}
	return "", fmt.Errorf("单章超过 40 页")
}

// nextChapterPage 从页面脚本文本或翻页链接中推断章内下一页，并验证目标仍在同一站点。
// 脚本规则只读取符合模式的文本。链接规则根据地址的页码后缀或「下一页」文字识别章内翻页。
func (d novelDownload) nextChapterPage(doc *html.Node, base *url.URL) string {
	rule := d.source.Crawler.Chapter
	links := novelSelect(doc, rule.NextPage)
	if rule.ScriptNext != "" {
		for _, node := range novelSelect(doc, "script") {
			if node.FirstChild == nil {
				continue
			}
			script := node.FirstChild.Data
			if rule.ScriptNext == "nextpage" {
				if m := scriptNextPage.FindStringSubmatch(script); len(m) > 1 {
					if next := novelLink(base, m[1]); next != "" {
						return next
					}
				}
			}
			if rule.ScriptNext == "keyboard" {
				if m := scriptNextVariable.FindStringSubmatch(script); len(m) > 1 {
					variable := regexp.MustCompile(`\bvar\s+` + regexp.QuoteMeta(m[1]) + `\s*=\s*['"]([^'"]+)['"]`)
					if ref := variable.FindStringSubmatch(script); len(ref) > 1 && novelPageSuffix.MatchString(ref[1]) {
						return novelLink(base, ref[1])
					}
				}
			}
		}
	}
	for _, node := range links {
		next := novelLink(base, attribute(node, "href"))
		if next == "" || next == base.String() {
			continue
		}
		u, _ := url.Parse(next)
		path := novelPageSuffix.ReplaceAllString(u.Path, "$1")
		current := novelPageSuffix.ReplaceAllString(base.Path, "$1")
		label := elementText(node)
		if path == current || strings.Contains(label, "下一页") || strings.Contains(label, "下頁") || strings.Contains(label, "下一頁") {
			return next
		}
	}
	return ""
}

// contentText 把命中的正文容器转换为纯文本，按配置移除广告元素或解码嵌入式正文，再整理段落。
func (d novelDownload) contentText(doc *html.Node, filter *regexp.Regexp) string {
	rule := d.source.Crawler.Chapter
	var text strings.Builder
	for _, content := range novelSelect(doc, rule.Content) {
		if rule.Base64 {
			// 从指定脚本调用中取出字符串，解码后仍按 HTML 处理换行和标签。
			for _, script := range novelSelect(content, "script") {
				if script.FirstChild == nil {
					continue
				}
				for _, match := range novelBase64Text.FindAllStringSubmatch(script.FirstChild.Data, -1) {
					raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(match[1], "="))
					if err == nil {
						if decoded, err := parseHTML(raw); err == nil {
							writeNovelText(&text, decoded)
							text.WriteByte('\n')
						}
					}
				}
			}
			continue
		}
		for _, node := range novelSelect(content, rule.FilterTag) {
			if node.Parent != nil {
				node.Parent.RemoveChild(node)
			}
		}
		writeNovelText(&text, content)
		text.WriteByte('\n')
	}
	body := text.String()
	if filter != nil {
		body = filter.ReplaceAllString(body, "")
	}
	var lines []string
	for line := range strings.SplitSeq(body, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n\n")
}

// writeNovelText 递归收集节点文本，以块级元素和 br 划分段落，并跳过脚本、样式及头部内容。
func writeNovelText(text *strings.Builder, node *html.Node) {
	if node.Type == html.TextNode {
		text.WriteString(node.Data)
		return
	}
	switch node.Data {
	case "script", "style", "head", "noscript":
		return
	case "br":
		text.WriteByte('\n')
		return
	case "p", "div", "section", "li", "h1", "h2", "h3":
		text.WriteByte('\n')
		defer text.WriteByte('\n')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeNovelText(text, child)
	}
}

// ValidateCrawlerDownload 核对请求与已启用的抓取书源是否匹配，并校验详情地址和 EPUB 文件名。
func ValidateCrawlerDownload(source Source, file File) error {
	if !source.Enabled || source.Kind != CrawlerKind || source.Crawler == nil || file.NovelSourceID == "" ||
		file.NovelSourceID != source.ID || !source.isBookURL(file.URL) || !strings.EqualFold(filepath.Ext(file.Name), ".epub") {
		return fmt.Errorf("无效的章节书源或书籍详情链接")
	}
	if file.Concurrency < 0 || file.Concurrency > maxCrawlerConcurrency {
		return fmt.Errorf("章节抓取协程数需在 1–%d 之间", maxCrawlerConcurrency)
	}
	return nil
}

// DownloadCrawler 抓取详情、封面、目录和正文，在任务临时目录生成 EPUB。
// progress 可为空，参数依次为已写入章节数和总章节数。
func DownloadCrawler(ctx context.Context, client *http.Client, source Source, file File, dir string, progress func(int64, int64)) (string, error) {
	if err := ValidateCrawlerDownload(source, file); err != nil {
		return "", err
	}
	rule := source.Crawler
	pace := time.NewTicker(time.Duration(max(rule.MinInterval, 300)) * time.Millisecond)
	defer pace.Stop()
	download := novelDownload{source: source, client: newNovelClient(client), tick: pace.C}
	doc, base, err := download.page(ctx, file.URL)
	if err != nil {
		return "", fmt.Errorf("读取书籍详情失败: %w", err)
	}
	meta, coverURL := source.crawlerEPUBMetadata(doc, base, file)
	chapters, err := download.chapters(ctx, file.URL, doc, base)
	if err != nil {
		return "", err
	}
	if progress != nil {
		// 拿到完整目录后才能确定总章数，先通知界面从读取目录切换到章节进度。
		progress(0, int64(len(chapters)))
	}
	cover, err := download.cover(ctx, coverURL, meta.Source)
	if err != nil {
		return "", fmt.Errorf("准备 EPUB 封面失败: %w", err)
	}
	target := filepath.Join(dir, SafeFileName(file.Name, ".epub"))
	titles := make([]string, len(chapters))
	directory := make(map[string]bool, len(chapters))
	for i, chapter := range chapters {
		titles[i] = chapter.title
		directory[chapter.target] = true
	}
	var filter *regexp.Regexp
	if rule.Chapter.FilterTxt != "" {
		filter, err = regexp.Compile(rule.Chapter.FilterTxt)
		if err != nil {
			return "", err
		}
	}
	// chapterResult 保存单章抓取结果，按其在批次中的位置汇总。
	type chapterResult struct {
		// body 是合并了章内分页的正文。
		body string
		// err 保存最终一次抓取的错误，成功时为空。
		err error
	}
	width := min(max(rule.Concurrency, 1), 4)
	if source.Group != defaultSourceGroup && file.Concurrency > 0 {
		width = file.Concurrency
	}
	var written int64
	var results []chapterResult
	err = epubexport.Write(target, meta, titles, cover, func(index int) ([]string, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// 导出器写完上一章才请求下一章，因此 index 等于已写入的章数。
		if index > 0 && progress != nil {
			progress(int64(index), int64(len(chapters)))
		}
		// 导出器按目录顺序索要正文。进入新批次时并行抓取，内存只保留当前批次。
		offset := index % width
		if offset == 0 {
			group := chapters[index:min(index+width, len(chapters))]
			results = make([]chapterResult, len(group))
			var workers sync.WaitGroup
			for i, chapter := range group {
				workers.Go(func() {
					// 每章最多重试一次，所有请求仍共用任务限速器。
					for attempt := 0; attempt < 2; attempt++ {
						body, err := download.chapterText(ctx, chapter, directory, filter)
						results[i] = chapterResult{body, err}
						if err == nil || ctx.Err() != nil {
							break
						}
					}
				})
			}
			workers.Wait()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		result := results[offset]
		if result.err != nil {
			return nil, result.err
		}
		written += int64(len(titles[index]) + len(result.body))
		if written > MaxDownloadBytes {
			return nil, fmt.Errorf("电子书正文超过 100 MiB")
		}
		return strings.Split(result.body, "\n\n"), nil
	})
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(target)
		return "", err
	}
	if info, err := os.Stat(target); err != nil {
		return "", err
	} else if info.Size() > MaxDownloadBytes {
		_ = os.Remove(target)
		return "", fmt.Errorf("生成的 EPUB 超过 100 MiB")
	}
	if progress != nil {
		progress(int64(len(chapters)), int64(len(chapters)))
	}
	return target, nil
}
