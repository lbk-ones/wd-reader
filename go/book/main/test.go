package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"regexp"
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

type Hitokoto struct {
	Id         int     `json:"id"`
	Uuid       string  `json:"uuid"`
	Hitokoto   string  `json:"hitokoto"`
	Type       string  `json:"type"`
	From       string  `json:"from"`
	FromWho    *string `json:"from_who"`
	Creator    string  `json:"creator"`
	CreatorUid int     `json:"creator_uid"`
	Reviewer   int     `json:"reviewer"`
	CommitFrom string  `json:"commit_from"`
	CreatedAt  string  `json:"created_at"`
	Length     int     `json:"length"`
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
	//findString := constant.RegChapter.FindString("1")
	//fmt.Println(findString)

	var w []string
	for c := 'a'; c <= 'l'; c++ {
		fmt.Println("开始", string(c))
		join := path.Join("D:\\GolandProjects\\wd-reader\\sentences", fmt.Sprintf("%c.json", c))
		open, _ := os.Open(join)
		all, _ := io.ReadAll(open)
		var hitokotos []Hitokoto
		err := json.Unmarshal(all, &hitokotos)
		if err != nil {
			fmt.Println(err)
		}
		for _, item := range hitokotos {
			hitokoto := item.Hitokoto
			if len(hitokoto) < 50 {
				w = append(w, hitokoto)
			}
		}
	}

	create, _ := os.Create("D:\\GolandProjects\\wd-reader\\sentences\\sum.txt")
	create.WriteString(strings.Join(w, "\n"))

	//fmt.Println(string(all))
	//extract := book.GetChapterListByFileNameExtract("很纯很暧昧.txt")
	//fmt.Println(extract)
}
