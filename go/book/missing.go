package book

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type MissingBook struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	FileName     string   `json:"fileName"`
	OriginalName string   `json:"originalName"`
	CanRestore   bool     `json:"canRestore"`
	Candidates   []string `json:"candidates"`
}

func ListMissingBooks() ([]MissingBook, error) {
	if _, err := ListMetadata(); err != nil {
		return nil, err
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return nil, err
	}
	result := []MissingBook{}
	for _, b := range s.Books {
		if !validBookName(b.FileName) {
			continue
		}
		if _, err := os.Stat(filepath.Join(ParsePath(), b.FileName)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		item := MissingBook{ID: b.ID, Title: b.Title, FileName: b.FileName, OriginalName: b.OriginalName, Candidates: []string{}}
		if validBookName(b.OriginalName) && strings.TrimSuffix(b.OriginalName, filepath.Ext(b.OriginalName))+".txt" == b.FileName {
			if info, e := os.Stat(filepath.Join(PurePath(), b.OriginalName)); e == nil && info.Mode().IsRegular() {
				item.CanRestore = true
			}
		}
		if b.FileHash != "" {
			for _, candidate := range s.Books {
				if candidate.ID == b.ID || candidate.FileHash != b.FileHash || !validBookName(candidate.FileName) {
					continue
				}
				if info, e := os.Stat(filepath.Join(ParsePath(), candidate.FileName)); e == nil && info.Mode().IsRegular() {
					item.Candidates = append(item.Candidates, candidate.FileName)
				}
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func RelinkMissingBook(id, target string) error {
	if !validBookName(target) || !strings.EqualFold(filepath.Ext(target), ".txt") {
		return fmt.Errorf("无效的候选文件")
	}
	if err := EnsureStorage(); err != nil {
		return err
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	oldIndex, targetIndex := -1, -1
	for i, b := range s.Books {
		if b.ID == id {
			oldIndex = i
		}
		if b.FileName == target {
			targetIndex = i
		}
	}
	if oldIndex < 0 {
		return fmt.Errorf("缺失记录不存在")
	}
	old := s.Books[oldIndex]
	if !validBookName(old.FileName) {
		return fmt.Errorf("无效的缺失记录")
	}
	if _, err = os.Stat(filepath.Join(ParsePath(), old.FileName)); err == nil {
		return fmt.Errorf("原文件已恢复，请刷新")
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.Open(filepath.Join(ParsePath(), target))
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(h, file)
	file.Close()
	if err != nil {
		return err
	}
	if old.FileHash == "" || fmt.Sprintf("%x", h.Sum(nil)) != old.FileHash {
		return fmt.Errorf("候选文件内容与原记录不一致")
	}
	if targetIndex >= 0 {
		candidate := s.Books[targetIndex]
		if candidate.ID == old.ID || candidate.Cover != "" || candidate.OriginalName != "" || len(candidate.Authors) > 0 || candidate.Description != "" || len(candidate.Sources) > 0 || candidate.Title != strings.TrimSuffix(target, filepath.Ext(target)) {
			return fmt.Errorf("候选文件已有书籍信息，不能覆盖；请先处理该书")
		}
		if candidate.LastOpenedAt.After(old.LastOpenedAt) {
			old.LastOpenedAt = candidate.LastOpenedAt
		}
	}
	old.FileName = target
	info, err := os.Stat(filepath.Join(ParsePath(), target))
	if err != nil {
		return err
	}
	old.FileSize, old.FileModTime = info.Size(), info.ModTime().UnixNano()
	books := make([]BookMetadata, 0, len(s.Books))
	for i, b := range s.Books {
		if i != oldIndex && i != targetIndex {
			books = append(books, b)
		}
	}
	s.Books = append(books, old)
	return writeMetadata(s)
}

func DeleteMissingBook(id string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i, b := range s.Books {
		if b.ID != id {
			continue
		}
		if !validBookName(b.FileName) {
			return fmt.Errorf("无效的缺失记录")
		}
		if _, err := os.Stat(filepath.Join(ParsePath(), b.FileName)); err == nil {
			return fmt.Errorf("文件已恢复，请刷新")
		} else if !os.IsNotExist(err) {
			return err
		}
		s.Books = append(s.Books[:i], s.Books[i+1:]...)
		if err = writeMetadata(s); err != nil {
			return err
		}
		if b.Cover != "" && filepath.Base(b.Cover) == b.Cover {
			used := false
			for _, other := range s.Books {
				if other.Cover == b.Cover {
					used = true
					break
				}
			}
			if !used {
				_ = os.Remove(filepath.Join(metadataDir(), "covers", b.Cover))
			}
		}
		return nil
	}
	return fmt.Errorf("缺失记录不存在")
}
