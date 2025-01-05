package main

import (
	"fmt"
	"wd-reader/go/book"

	//"fmt"
	//"path"
	"regexp"
	"wd-reader/go/constant"
	//"wd-reader/go/book/epub/EpubBook"
)

func extractFileNames(inputs []string) []string {
	re := regexp.MustCompile(`[^#]+(\.html|\.xhtml)`)
	var results []string
	for _, input := range inputs {
		result := re.FindString(input)
		if result != "" {
			results = append(results, result)
		}
	}
	return results
}

func main() {
	//EpubBook.GetChapterListByFileNameExtract("岳父朱棣，迎娶毁容郡主我乐麻了【正文完结版】 (过节长肉肉) (Z-Library).txt")
	//epub, err := EpubBook.ParseEpub("C:\\Users\\win 10\\Downloads\\hcham.epub", "D:\\GolandProjects\\wd-reader\\books\\很纯很暧昧.txt")
	//if err != nil {
	//	fmt.Println(err)
	//	panic(err)
	//}
	//err = epub.WriteEpub()
	//if err != nil {
	//	fmt.Println(err)
	//	panic(err)
	//}
	//fmt.Println("写入成功" + strconv.Itoa(len(epub.Sections)) + "目录")
	//var wt = "http://agagha/agahga.txt"
	//base := path.Base(wt)
	//println(base)
	//if err != nil {
	//	fmt.Println(err)
	//} else {
	//for _, name := range epub.CatLogs {
	//	fmt.Println(name.Title)
	//}
	//
	//sort.Slice(epub.Sections, func(i, j int) bool {
	//	return epub.Sections[i].PlayOrder < epub.Sections[j].PlayOrder
	//})
	//for in, name := range epub.Sections {
	//	fmt.Println(fmt.Sprint(in+1) + "title--------" + name.Title)
	//	//fmt.Println()
	//}
	//}
	findString := constant.RegChapter.FindString("第T15章 番外特别章（2）")
	fmt.Println(findString)

	extract := book.GetChapterListByFileNameExtract("很纯很暧昧.txt")
	fmt.Println(extract)
}
