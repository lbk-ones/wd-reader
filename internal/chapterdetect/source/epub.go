package source

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

// epubPackage 提取分章所需的内容清单和阅读顺序，不承载封面、作者等无关元数据。
type epubPackage struct {
	// Items 为清单资源，资源标识用于关联阅读顺序，属性用于寻找导航文件。
	Items []struct {
		ID         string `xml:"id,attr"`         // 清单中的唯一资源标识。
		Href       string `xml:"href,attr"`       // 相对内容清单文件的资源路径。
		MediaType  string `xml:"media-type,attr"` // 媒体类型，用于兼容旧版导航文件。
		Properties string `xml:"properties,attr"` // 资源属性，包含新版导航文件标记。
	} `xml:"manifest>item"`
	// Spine 指定正文的阅读顺序，不能用压缩包内文件顺序代替。
	Spine struct {
		TOC string `xml:"toc,attr"` // 旧版目录所引用的清单资源标识。
		// Items 是有序的正文资源引用。
		Items []struct {
			ID string `xml:"idref,attr"` // 对应清单项的资源标识。
		} `xml:"itemref"`
	} `xml:"spine"`
}

// navPoint 对应旧版嵌套目录中的一个节点，通过递归子节点保留目录层级顺序。
type navPoint struct {
	Title string `xml:"navLabel>text"` // 原生目录标题。
	// Content 为目标位置，可同时包含正文文件名及文件内片段。
	Content struct {
		Src string `xml:"src,attr"` // 相对目录文件的链接目标。
	} `xml:"content"`
	Children []navPoint `xml:"navPoint"` // 按目录顺序排列的下级条目。
}

// resolveHref 将目录或清单中的相对链接解析为压缩包内路径及已解码的片段标识。
// base 为链接所在文件；拒绝外部地址、反斜杠和越过压缩包根目录的路径。
func resolveHref(base, href string) (filename, fragment string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil || u.Scheme != "" || u.Host != "" || strings.Contains(u.Path, "\\") {
		return "", "", false
	}
	filename = base
	if u.Path != "" {
		filename = path.Clean(path.Join(path.Dir(base), u.Path))
	}
	if filename == "." || strings.HasPrefix(filename, "../") || strings.HasPrefix(filename, "/") {
		return "", "", false
	}
	return filename, u.Fragment, true
}

// ReadEPUB 按内容清单的阅读顺序提取正文，使用新版导航或旧版嵌套目录定位原生章节。
// 同一正文文件内的多个片段可分别生成章节；目录不可用时仍返回正文，交由上层文本检测。
// 文件读取、结构解码或正文提取失败时返回错误，不以不完整正文替代完整导入结果。
func ReadEPUB(filename string) (Document, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return Document{}, err
	}
	defer archive.Close()
	// 限制单个解压条目的读取量，防止异常资源占用过量内存；整本正文另有累计上限。
	read := func(name string) ([]byte, error) {
		file, err := archive.Open(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
		if err == nil && len(data) > 64<<20 {
			err = fmt.Errorf("EPUB 内容文件过大: %s", name)
		}
		return data, err
	}
	container, err := read("META-INF/container.xml")
	if err != nil {
		return Document{}, err
	}
	// 从标准容器文件定位内容清单，而不是假定清单具有固定文件名。
	var roots struct {
		// Files 为容器声明的根文件，当前读取第一个声明。
		Files []struct {
			Path string `xml:"full-path,attr"` // 内容清单在压缩包内的完整路径。
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(container, &roots); err != nil || len(roots.Files) == 0 {
		return Document{}, fmt.Errorf("EPUB 缺少有效的内容清单")
	}
	opfName := path.Clean(roots.Files[0].Path)
	raw, err := read(opfName)
	if err != nil {
		return Document{}, err
	}
	var pkg epubPackage
	if err := xml.Unmarshal(raw, &pkg); err != nil {
		return Document{}, err
	}
	// 建立资源标识到压缩包路径的映射，并识别新版导航与旧版目录资源。
	files := map[string]string{}
	navFiles := map[string]bool{}
	var navName, ncxName string
	for _, item := range pkg.Items {
		name, _, ok := resolveHref(opfName, item.Href)
		if !ok {
			continue
		}
		files[item.ID] = name
		if strings.Contains(" "+item.Properties+" ", " nav ") {
			navName = name
			navFiles[name] = true
		}
		if item.ID == pkg.Spine.TOC || (ncxName == "" && item.MediaType == "application/x-dtbncx+xml") {
			ncxName = name
		}
	}
	// 目标索引依次按正文文件、片段标识组织；同一位置可能有父级和子级两个标题。
	targets := map[string]map[string][]string{}
	add := func(base string, entry link) {
		name, fragment, ok := resolveHref(base, entry.href)
		if !ok || strings.TrimSpace(entry.title) == "" {
			return
		}
		if targets[name] == nil {
			targets[name] = map[string][]string{}
		}
		targets[name][fragment] = append(targets[name][fragment], entry.title)
	}
	// 优先使用新版导航；只有没有收集到目标时，才回退到旧版嵌套目录。
	if navName != "" {
		if data, err := read(navName); err == nil {
			var entries []link
			if doc, err := html.Parse(strings.NewReader(string(data))); err == nil {
				entries = tocLinks(doc)
			} else {
				entries = streamTOCLinks(string(data))
			}
			for _, entry := range entries {
				add(navName, entry)
			}
		}
	}
	if len(targets) == 0 && ncxName != "" {
		if data, err := read(ncxName); err == nil {
			var ncx struct {
				Points []navPoint `xml:"navMap>navPoint"` // 按原书顺序排列的顶层目录节点。
			}
			if xml.Unmarshal(data, &ncx) == nil {
				var visit func([]navPoint)
				visit = func(points []navPoint) {
					for _, point := range points {
						add(ncxName, link{point.Content.Src, point.Title})
						visit(point.Children)
					}
				}
				visit(ncx.Points)
			}
		}
	}
	// 严格按照正文阅读顺序渲染；重复引用的文件仅处理一次，避免目录和正文重复。
	builder := &textBuilder{}
	visited := map[string]bool{}
	total := 0
	for _, item := range pkg.Spine.Items {
		name := files[item.ID]
		if name == "" {
			return Document{}, fmt.Errorf("EPUB 阅读顺序引用了缺失的文件")
		}
		if visited[name] {
			continue
		}
		visited[name] = true
		data, err := read(name)
		if err != nil {
			return Document{}, err
		}
		total += len(data)
		if total > 512<<20 {
			return Document{}, fmt.Errorf("EPUB 正文超过解析限制")
		}
		// 正常文件使用文档树；过深或异常标签结构改用流式提取，同样映射原生锚点。
		doc, err := html.Parse(strings.NewReader(string(data)))
		if err != nil {
			renderHTMLStream(builder, string(data), targets[name])
		} else {
			renderHTML(builder, doc, targets[name], navFiles[name])
		}
	}
	result := builder.finish()
	if len(result.Lines) == 0 {
		return Document{}, fmt.Errorf("EPUB 未提取到正文")
	}
	return result, nil
}
