package EpubBook

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	_type "wd-reader/go/book/epub/type"
	"wd-reader/go/constant"
	"wd-reader/go/utils"
)

type EpubBook struct {
	Title       string
	Author      string
	Description string
	Date        string
	Publisher   string
	Language    string
	InputPath   string
	OutputPath  string
	Sections    []Section
	CatLogs     []CatLog
	ZipReader   *zip.ReadCloser
	Ncx         *_type.NCX
	NcxPath     string
	OpfPath     string
	Opf         *_type.Package
	NcxFile     fs.File
	OpfFile     fs.File
}

// ParseEpub 解析Epub文件
func ParseEpub(path string, outOutPath string) (*EpubBook, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, err
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	b2 := &EpubBook{
		ZipReader:   reader,
		Title:       "",
		Author:      "",
		Description: "",
		InputPath:   path,
		OutputPath:  outOutPath,
		NcxFile:     nil,
		OpfFile:     nil,
		Ncx:         nil,
		Sections:    nil,
		CatLogs:     nil,
	}
	err2 := initParse(err, b2)
	if err2 != nil {
		return nil, err2
	}

	b2.Title = getTitle(b2)
	b2.Author = getAuthor(b2)
	b2.Description = html.UnescapeString(b2.Opf.Metadata.Description)
	b2.Date = html.UnescapeString(b2.Opf.Metadata.Date.Value)
	b2.Publisher = html.UnescapeString(b2.Opf.Metadata.Publisher)
	b2.Language = html.UnescapeString(b2.Opf.Metadata.Language)

	// parse catlog list
	parseCatLog(b2)
	//err = checkCatLog(b2)
	if err != nil {
		return nil, err
	}
	parseChaptersV2(b2)

	// close resource
	defer func() {
		err := b2.NcxFile.Close()
		if err != nil {
			fmt.Println(err)
		}
		err = b2.OpfFile.Close()
		if err != nil {
			return
		}
		err = reader.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()

	return b2, nil

}

func checkCatLog(b2 *EpubBook) error {
	m := map[string]string{}
	for _, cat := range b2.CatLogs {
		link := cat.Link
		s := m[link]
		if s != "" {
			return errors.New("epub文件异常或损坏，有重复目录")
		}
		m[link] = "ok"
	}

	return nil

}

// WriteEpub write to file
func (b *EpubBook) WriteEpub() error {

	if b.OutputPath != "" {
		create, err := os.Create(b.OutputPath)
		if err != nil {
			return err
		}

		fileName := b.Title
		_, _ = create.WriteString("0、文本元信息\n\n")
		_, _ = create.WriteString("Title       :" + fileName + "\n")
		author := b.Author
		_, _ = create.WriteString("Author      :" + author + "\n")
		description := (func() string {
			if b.Description == "" {
				return "无"
			} else {
				return b.Description
			}
		})()
		_, _ = create.WriteString("Description :" + description + "\n")
		Date := b.Date
		_, _ = create.WriteString("Date        :" + Date + "\n")
		Language := b.Language
		_, _ = create.WriteString("Language    :" + Language + "\n")
		Publisher := b.Publisher
		_, _ = create.WriteString("Publisher   :" + Publisher + "\n")

		_, _ = create.WriteString("\n")
		for _, line := range b.Sections {
			_, _ = create.WriteString(" \n\n")
			//_, _ = create.WriteString(line.Title + "\n\n")
			_, _ = create.WriteString(line.Content)
			_, _ = create.WriteString(" \n\n")
		}

		defer func(create *os.File) {
			err := create.Close()
			if err != nil {
				fmt.Println(err)
			}
		}(create)
	}
	return nil
}

func extractFileNames(input string) string {
	re := regexp.MustCompile(`[^#]+(\.html|\.xhtml)`)
	result := re.FindString(input)
	return result
}

// direct parse opf file
func parseChaptersV2(b2 *EpubBook) {
	itemRefs := utils.NewArrayListElements[_type.ItemRef](b2.Opf.Spine.ItemRefs)
	manifestItems := utils.NewArrayListElements[_type.Item](b2.Opf.Manifest.Items)
	catLogs := utils.NewArrayListElements[CatLog](b2.CatLogs)
	opfPath := b2.OpfPath
	var sections []Section
	itemRefs.ForEach(func(index int, item _type.ItemRef) bool {
		ref := item.IDRef
		find := manifestItems.Find(func(item _type.Item) bool {
			return item.ID == ref
		})
		// exists
		if find.ID != "" {
			href := find.Href
			names := extractFileNames(href)
			chapterFile := opfPath + "/" + names
			if strings.Index(chapterFile, "/") == 0 {
				chapterFile = strings.Replace(chapterFile, "/", "", 1)
			}

			chapterZip, err := b2.ZipReader.Open(chapterFile)
			if err != nil {
				fmt.Printf("Error opening chapter file %s: %v\n", chapterFile, err)
				return true
			}

			doc, err := html.Parse(chapterZip)
			if err != nil {
				_ = chapterZip.Close()
				fmt.Printf("Error parsing chapter file %s: %v\n", chapterFile, err)
				return true
			}

			// extract text from html doc
			var extractText func(*html.Node) string
			extractText = func(n *html.Node) string {
				var s string
				if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "head" || n.Data == "title" || n.Data == "script" || n.Data == "style" || n.Data == "noscript" || n.Data == "meta") {
					return ""
				} else if n.Type == html.TextNode {
					s = n.Data
				}
				var tag string
				if n.Type == html.ElementNode {
					tag = n.Data
				}
				// recursion
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					s += extractText(c)
				}
				// these tag is not allow wrap
				if tag == "p" || tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" {
					s = strings.ReplaceAll(s, "\n", "") + "\n"
				}
				return s
			}

			text := extractText(doc)
			text = html.UnescapeString(text)
			re := regexp.MustCompile(`\n+`)
			result := re.ReplaceAllString(text, "\n")
			split := strings.Split(result, "\n")
			var newContent []string
			var title string
			for _, line := range split {
				// first line , maybe title
				line2 := strings.TrimSpace(line)
				if line2 != "\n" && line2 != "" {
					//firstLineNoSpace := strings.ReplaceAll(line, " ", "")
					log := catLogs.Find(func(point CatLog) bool {
						title := point.Title
						return title == line2
					})
					if title == "" && log.Title != "" {
						title = log.Title
					}
					//all := strings.ReplaceAll(log.Title, " ", "")
					//if all != firstLineNoSpace || all == "" {
					//
					//}

					newContent = append(newContent, line)
				}
			}
			text = strings.Join(newContent, "\n")
			if text != "" {
				s := Section{
					Title: (func() string {
						if title != "" {
							return title
						} else {
							return fmt.Sprint(index, "、")
						}
					})(),
					Content:   text,
					PlayOrder: index,
				}
				//fmt.Println("内容---" + text)
				sections = append(sections, s)
			}
			_ = chapterZip.Close()
		}
		return true
	})
	b2.Sections = sections

}

// Deprecated
func parseChaptersV1(b2 *EpubBook) {

	fmt.Println("-------------开始解析章节")
	logs := b2.CatLogs
	refs := b2.Opf.Spine.ItemRefs
	list := utils.NewArrayList[_type.ItemRef]()
	list.AddAll(refs)
	var sections []Section
	// take .ncx file path
	ncxPath := b2.NcxPath
	for in, log := range logs {
		title := log.Title
		fmt.Println(fmt.Sprint(in) + "-------------开始解析章节--" + title)
		// handler chapter title
		findString := constant.RegChapter.FindString(title)
		if findString == "" {
			title = fmt.Sprint(in+1, "、") + title
		}
		link := log.Link
		names := extractFileNames(link)
		chapterFile := ncxPath + "/" + names
		if strings.Index(chapterFile, "/") == 0 {
			chapterFile = strings.Replace(chapterFile, "/", "", 1)
		}
		chapterZip, err := b2.ZipReader.Open(chapterFile)
		if err != nil {
			fmt.Printf("Error opening chapter file %s: %v\n", chapterFile, err)
			continue
		}

		doc, err := html.Parse(chapterZip)
		if err != nil {
			_ = chapterZip.Close()
			fmt.Printf("Error parsing chapter file %s: %v\n", chapterFile, err)
			continue
		}

		// extract text from html doc
		var extractText func(*html.Node) string
		extractText = func(n *html.Node) string {
			var s string
			if n.Type == html.ElementNode && (n.Data == "head" || n.Data == "title" || n.Data == "script" || n.Data == "style" || n.Data == "noscript" || n.Data == "meta") {
				return ""
			} else if n.Type == html.TextNode {
				s = n.Data
			}
			var tag string
			if n.Type == html.ElementNode {
				tag = n.Data
			}
			// recursion
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				s += extractText(c)
			}
			// these tag is not allow wrap
			if tag == "p" || tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" || tag == "h5" {
				s = strings.ReplaceAll(s, "\n", "") + "\n"
			}

			return s
		}

		titleNoSpace := strings.ReplaceAll(title, " ", "")

		text := extractText(doc)
		text = html.UnescapeString(text)
		re := regexp.MustCompile(`\n+`)
		result := re.ReplaceAllString(text, "\n")
		split := strings.Split(result, "\n")
		var newContent []string
		for _, line := range split {
			// first line , maybe title
			line2 := strings.TrimSpace(line)
			if line2 != "\n" && line2 != "" {
				firstLineNoSpace := strings.ReplaceAll(line, " ", "")
				if titleNoSpace != firstLineNoSpace {
					newContent = append(newContent, line)
				}
			}
		}
		text = strings.Join(newContent, "\n")
		s := Section{
			Title:     title,
			Content:   text,
			PlayOrder: log.PlayOrder,
		}
		fmt.Println("内容---" + text)
		sections = append(sections, s)
		_ = chapterZip.Close()

	}
	b2.Sections = sections
}

func parseCatLog(b2 *EpubBook) {

	fmt.Println("----------开始解析目录--------------")
	ncx := b2.Ncx
	points := ncx.NavMap.NavPoints
	listMulu := utils.NewArrayList[CatLog]()
	for _, ite := range points {
		text := ite.NavLabel.Text
		log := CatLog{
			Id:        ite.ID,
			Title:     text,
			PlayOrder: ite.PlayOrder,
			Link:      ite.Content.Src,
		}
		//fmt.Println("---" + text)
		listMulu.Add(log)
		var parsePoint func([]_type.NavPoint)
		parsePoint = func(points []_type.NavPoint) {
			for _, po := range points {
				text2 := po.NavLabel.Text
				log := CatLog{
					Id:        po.ID,
					Title:     text2,
					PlayOrder: po.PlayOrder,
					Link:      po.Content.Src,
				}
				//fmt.Println("---" + text2)
				listMulu.Add(log)

				if len(po.Points) > 0 {
					parsePoint(po.Points)
				}
			}
		}
		if len(ite.Points) > 0 {
			parsePoint(ite.Points)
		}
	}
	b2.CatLogs = listMulu.GetElements()
	fmt.Println("----------目录解析完毕--------------")
}

func getAuthor(b2 *EpubBook) string {
	var resAuthor string
	metadata := b2.Opf.Metadata
	creator := metadata.Creator

	if len(creator) == 0 {
		resAuthor = "未知"
	} else {
		resAuthor = strings.Join(creator, ",")
	}
	return resAuthor
}

func getTitle(b2 *EpubBook) string {
	metadata := b2.Opf.Metadata
	title1 := strings.TrimSpace(metadata.Title)
	if title1 == "" || title1 == "UnKnown" || title1 == "未知" {
		ncx := b2.Ncx
		title1 = strings.TrimSpace(ncx.DocTitle.Text)
		if title1 == "" || title1 == "UnKnown" || title1 == "未知" {
			return filepath.Base(b2.InputPath)
		}
	}
	return title1
}

func initParse(err error, b2 *EpubBook) error {
	zipReader, err := getReaderByNameFromZipReader(b2, "ncx")
	if err != nil {
		return err
	}
	b2.NcxFile = zipReader
	err = parseNcx(b2)
	if err != nil {
		return err
	}

	opfFile, err := getReaderByNameFromZipReader(b2, "opf")
	if err != nil {
		return err
	}
	b2.OpfFile = opfFile
	err = parseOpf(b2)
	if err != nil {
		return err
	}
	return nil
}

func parseOpf(b2 *EpubBook) error {
	all, err := io.ReadAll(b2.OpfFile)
	if err != nil {
		return err

	}
	var obj *_type.Package
	err = xml.Unmarshal(all, &obj)
	if err != nil {
		return err
	}
	b2.Opf = obj
	return nil
}

func parseNcx(b2 *EpubBook) error {
	all, err := io.ReadAll(b2.NcxFile)
	if err == nil {
		var obj *_type.NCX
		err = xml.Unmarshal(all, &obj)
		if err != nil {
			return err
		}
		b2.Ncx = obj
	}
	return err

}

func getReaderByNameFromZipReader(fb *EpubBook, suffix string) (fs.File, error) {

	var ncxFilePath string
	for _, f := range fb.ZipReader.File {
		if strings.HasSuffix(f.Name, suffix) {
			ncxFilePath = f.Name
			break
		}
	}

	if ncxFilePath == "" {
		return nil, errors.New("未找到" + suffix + "文件")
	}

	dir := filepath.Dir(ncxFilePath)
	if dir == "." {
		dir = ""
	}
	if suffix == "ncx" {
		fb.NcxPath = dir
	}

	if suffix == "opf" {
		fb.OpfPath = dir
	}
	//chapterPath := dir + ncxFilePath

	reader, err2 := fb.ZipReader.Open(ncxFilePath)
	return reader, err2

}
