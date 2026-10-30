package book

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wd-reader/go/book/mobi"
	"wd-reader/go/constant"

	"github.com/google/uuid"
)

type BookMetadata struct {
	ID           string            `json:"id"`
	FileName     string            `json:"fileName"`
	ImportType   string            `json:"importType,omitempty"`
	OriginalName string            `json:"originalName,omitempty"`
	Title        string            `json:"title"`
	Authors      []string          `json:"authors"`
	Description  string            `json:"description"`
	Cover        string            `json:"cover"`
	CoverChecked bool              `json:"coverChecked,omitempty"`
	Sources      map[string]string `json:"sources"`
	ImportedAt   time.Time         `json:"importedAt"`
	LastOpenedAt time.Time         `json:"lastOpenedAt"`
	FileHash     string            `json:"fileHash,omitempty"`
	FileSize     int64             `json:"fileSize,omitempty"`
	FileModTime  int64             `json:"fileModTime,omitempty"`
}
type metadataStore struct {
	Version int            `json:"version"`
	Books   []BookMetadata `json:"books"`
}

var metadataMu sync.Mutex

func metadataDir() string { return filepath.Join(GetAppPath(), constant.BOOK_PATH, "meta-data") }

// Move the old sibling directory once, before reading or writing any metadata.
func ensureMetadataLocation() error {
	old := filepath.Join(GetAppPath(), constant.BOOK_PATH, "book-data")
	target := metadataDir()
	if _, err := os.Stat(old); os.IsNotExist(err) {
		old = filepath.Join(GetAppPath(), "book-data")
		if _, err = os.Stat(old); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		// Never overwrite a populated destination. An incomplete destination
		// must not cause the original metadata to be silently ignored.
		if _, err := os.Stat(filepath.Join(target, "metadata.json")); err != nil {
			return fmt.Errorf("无法迁移书籍信息：%s 已存在且没有 metadata.json；原数据保留在 %s", target, old)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := os.Rename(old, target); err != nil {
		return fmt.Errorf("迁移书籍信息失败: %w", err)
	}
	return nil
}
func readMetadata() (metadataStore, error) {
	s := metadataStore{Version: 1, Books: []BookMetadata{}}
	if err := ensureMetadataLocation(); err != nil {
		return s, err
	}
	b, err := os.ReadFile(filepath.Join(metadataDir(), "metadata.json"))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if s.Version != 1 {
		return s, fmt.Errorf("unsupported metadata version: %d", s.Version)
	}
	return s, nil
}
func writeMetadata(s metadataStore) error {
	if err := ensureMetadataLocation(); err != nil {
		return err
	}
	if err := os.MkdirAll(metadataDir(), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(metadataDir(), "metadata-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	target := filepath.Join(metadataDir(), "metadata.json")
	if old, e := os.ReadFile(target); e == nil {
		if e = os.WriteFile(target+".bak", old, 0644); e != nil {
			return e
		}
	}
	return os.Rename(name, target)
}
func validBookName(n string) bool { return n != "" && n == filepath.Base(n) && n != "." && n != ".." }
func importType(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}
func defaultMetadata(n string) BookMetadata {
	now := time.Now().UTC()
	return BookMetadata{ID: uuid.NewString(), FileName: n, ImportType: importType(n), Title: strings.TrimSuffix(n, filepath.Ext(n)), Authors: []string{}, Sources: map[string]string{}, ImportedAt: now, LastOpenedAt: now}
}

// RecordImport records a successful explicit import, including a replacement.
func RecordImport(name, originalName string) error {
	if !validBookName(name) {
		return fmt.Errorf("invalid book name")
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i := range s.Books {
		if s.Books[i].FileName == name {
			s.Books[i].ImportedAt = time.Now().UTC()
			s.Books[i].LastOpenedAt = s.Books[i].ImportedAt
			s.Books[i].ImportType = importType(originalName)
			s.Books[i].OriginalName = filepath.Base(originalName)
			return writeMetadata(s)
		}
	}
	b := defaultMetadata(name)
	b.ImportType = importType(originalName)
	b.OriginalName = filepath.Base(originalName)
	s.Books = append(s.Books, b)
	return writeMetadata(s)
}

func RecordBookOpened(name string) error {
	if !validBookName(name) {
		return fmt.Errorf("invalid book name")
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i := range s.Books {
		if s.Books[i].FileName == name {
			s.Books[i].LastOpenedAt = time.Now().UTC()
			return writeMetadata(s)
		}
	}
	return fmt.Errorf("书籍信息不存在")
}

// ListMetadata reconciles the index with files while preserving missing entries.
func ListMetadata() ([]BookMetadata, error) {
	if err := EnsureStorage(); err != nil {
		return nil, err
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(ParsePath())
	if err != nil {
		return nil, err
	}
	changed := false
	result := []BookMetadata{}
	present := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			present[e.Name()] = true
		}
	}
	type fingerprint struct {
		hash      string
		size, mod int64
	}
	fingerprints := map[string]fingerprint{}
	newCounts := map[string]int{}
	for _, e := range entries {
		n := e.Name()
		ext := strings.ToLower(filepath.Ext(n))
		if e.IsDir() || strings.HasPrefix(n, ".parse-") || ext != ".txt" {
			continue
		}
		info, e2 := e.Info()
		if e2 != nil {
			continue
		}
		known := false
		for _, b := range s.Books {
			if b.FileName == n && b.FileHash != "" && b.FileSize == info.Size() && b.FileModTime == info.ModTime().UnixNano() {
				fingerprints[n] = fingerprint{b.FileHash, b.FileSize, b.FileModTime}
				known = true
				break
			}
		}
		if !known {
			f, e2 := os.Open(filepath.Join(ParsePath(), n))
			if e2 != nil {
				continue
			}
			h := sha256.New()
			_, e2 = io.Copy(h, f)
			f.Close()
			if e2 != nil {
				continue
			}
			fingerprints[n] = fingerprint{fmt.Sprintf("%x", h.Sum(nil)), info.Size(), info.ModTime().UnixNano()}
		}
		found := false
		for _, b := range s.Books {
			if b.FileName == n {
				found = true
				break
			}
		}
		if !found {
			newCounts[fingerprints[n].hash]++
		}
	}
	for _, e := range entries {
		n := e.Name()
		ext := strings.ToLower(filepath.Ext(n))
		if e.IsDir() || strings.HasPrefix(n, ".parse-") || ext != ".txt" {
			continue
		}
		idx := -1
		for i := range s.Books {
			if s.Books[i].FileName == n {
				idx = i
				break
			}
		}
		if idx < 0 {
			fp := fingerprints[n]
			candidate := -1
			count := 0
			if fp.hash != "" && newCounts[fp.hash] == 1 {
				for i, b := range s.Books {
					if !present[b.FileName] && b.FileHash == fp.hash {
						candidate = i
						count++
					}
				}
			}
			if count == 1 {
				idx = candidate
				s.Books[idx].FileName = n
			} else {
				s.Books = append(s.Books, defaultMetadata(n))
				idx = len(s.Books) - 1
			}
			changed = true
		}
		if fp, ok := fingerprints[n]; ok && (s.Books[idx].FileHash != fp.hash || s.Books[idx].FileModTime != fp.mod || s.Books[idx].FileSize != fp.size) {
			s.Books[idx].FileHash = fp.hash
			s.Books[idx].FileSize = fp.size
			s.Books[idx].FileModTime = fp.mod
			changed = true
		}
		if s.Books[idx].ImportedAt.IsZero() {
			// Existing installations have no recorded import time; the file's
			// modification time is a best-effort historical approximation.
			if info, e2 := e.Info(); e2 == nil {
				s.Books[idx].ImportedAt = info.ModTime().UTC()
			} else {
				s.Books[idx].ImportedAt = time.Now().UTC()
			}
			changed = true
		}
		if s.Books[idx].LastOpenedAt.IsZero() {
			s.Books[idx].LastOpenedAt = s.Books[idx].ImportedAt
			changed = true
		}
		if s.Books[idx].OriginalName == "" {
			if info, e2 := os.Stat(filepath.Join(PurePath(), n)); e2 == nil && info.Mode().IsRegular() {
				s.Books[idx].OriginalName = n
				changed = true
			}
		}
		b := &s.Books[idx]
		if !b.CoverChecked && b.Cover == "" && b.Sources["cover"] != "user" && validBookName(b.OriginalName) {
			ext := strings.ToLower(filepath.Ext(b.OriginalName))
			if ext == ".mobi" || ext == ".azw" || ext == ".azw3" {
				source := filepath.Join(PurePath(), b.OriginalName)
				if info, e2 := os.Stat(source); e2 == nil && info.Mode().IsRegular() {
					if reader, e2 := mobi.NewReader(source); e2 == nil {
						if data := reader.ExtractCover(); len(data) > 0 {
							if cover, e2 := storeCover(data); e2 == nil {
								b.Cover = cover
								if b.Sources == nil {
									b.Sources = map[string]string{}
								}
								b.Sources["cover"] = "embedded"
							}
						}
					}
					b.CoverChecked = true
					changed = true
				}
			}
		}
		result = append(result, s.Books[idx])
	}
	if changed {
		err = writeMetadata(s)
	}
	return result, err
}
func UpdateMetadata(id, title string, authors []string, description string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i := range s.Books {
		b := &s.Books[i]
		if b.ID == id {
			b.Title = strings.TrimSpace(title)
			if b.Title == "" {
				return fmt.Errorf("书名不能为空")
			}
			b.Authors = authors
			b.Description = description
			if b.Sources == nil {
				b.Sources = map[string]string{}
			}
			for _, k := range []string{"title", "authors", "description"} {
				b.Sources[k] = "user"
			}
			return writeMetadata(s)
		}
	}
	return fmt.Errorf("书籍不存在")
}
func storeCover(data []byte) (string, error) {
	if len(data) > 10<<20 {
		return "", fmt.Errorf("封面不能超过 10 MB")
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("无效的封面图片")
	}
	ext := map[string]string{"jpeg": ".jpg", "png": ".png", "gif": ".gif"}[format]
	if ext == "" {
		return "", fmt.Errorf("不支持的封面格式")
	}
	sum := sha256.Sum256(data)
	name := fmt.Sprintf("%x%s", sum, ext)
	dir := filepath.Join(metadataDir(), "covers")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return name, os.WriteFile(filepath.Join(dir, name), data, 0644)
}
func SetCover(id, filename string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i := range s.Books {
		if s.Books[i].ID == id {
			cover := ""
			if filename != "" {
				info, e := os.Stat(filename)
				if e != nil {
					return e
				}
				if info.Size() > 10<<20 {
					return fmt.Errorf("封面不能超过 10 MB")
				}
				data, e := os.ReadFile(filename)
				if e != nil {
					return e
				}
				cover, e = storeCover(data)
				if e != nil {
					return e
				}
			}
			s.Books[i].Cover = cover
			if s.Books[i].Sources == nil {
				s.Books[i].Sources = map[string]string{}
			}
			s.Books[i].Sources["cover"] = "user"
			return writeMetadata(s)
		}
	}
	return fmt.Errorf("书籍不存在")
}
func CoverData(id string) (string, error) {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return "", err
	}
	for _, b := range s.Books {
		if b.ID == id && b.Cover != "" {
			if filepath.Base(b.Cover) != b.Cover {
				return "", fmt.Errorf("invalid cover")
			}
			data, e := os.ReadFile(filepath.Join(metadataDir(), "covers", b.Cover))
			if e != nil {
				return "", e
			}
			ext := strings.ToLower(filepath.Ext(b.Cover))
			mime := map[string]string{".jpg": "image/jpeg", ".png": "image/png", ".gif": "image/gif"}[ext]
			if mime == "" {
				return "", fmt.Errorf("invalid cover")
			}
			return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}
	return "", nil
}

func RemoveMetadata(filename string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i, b := range s.Books {
		if b.FileName != filename {
			continue
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
	return nil
}

func CaptureMOBIMetadata(target, title string, authors []string, description string) error {
	if !validBookName(target) {
		return fmt.Errorf("invalid book name")
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	idx := -1
	for i := range s.Books {
		if s.Books[i].FileName == target {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.Books = append(s.Books, defaultMetadata(target))
		idx = len(s.Books) - 1
	}
	b := &s.Books[idx]
	if b.Sources == nil {
		b.Sources = map[string]string{}
	}
	if strings.TrimSpace(title) != "" && b.Sources["title"] != "user" {
		b.Title = strings.TrimSpace(title)
		b.Sources["title"] = "embedded"
	}
	if len(authors) > 0 && b.Sources["authors"] != "user" {
		b.Authors = authors
		b.Sources["authors"] = "embedded"
	}
	if description != "" && b.Sources["description"] != "user" {
		b.Description = description
		b.Sources["description"] = "embedded"
	}
	return writeMetadata(s)
}

// CaptureEPUBMetadata runs before the original EPUB is deleted during conversion.
func CaptureEPUBMetadata(source, target string) error {
	if !validBookName(target) {
		return fmt.Errorf("invalid book name")
	}
	z, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer z.Close()
	read := func(n string, limit int64) ([]byte, error) {
		for _, f := range z.File {
			if f.Name == n {
				if f.UncompressedSize64 > uint64(limit) {
					return nil, fmt.Errorf("EPUB entry too large")
				}
				r, e := f.Open()
				if e != nil {
					return nil, e
				}
				defer r.Close()
				return io.ReadAll(io.LimitReader(r, limit+1))
			}
		}
		return nil, os.ErrNotExist
	}
	container, e := read("META-INF/container.xml", 1<<20)
	if e != nil {
		return e
	}
	var c struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if e = xml.Unmarshal(container, &c); e != nil || len(c.Rootfiles) == 0 {
		return fmt.Errorf("invalid EPUB container")
	}
	opfPath := c.Rootfiles[0].FullPath
	raw, e := read(opfPath, 4<<20)
	if e != nil {
		return e
	}
	var p struct {
		Metadata struct {
			Title       string   `xml:"title"`
			Creators    []string `xml:"creator"`
			Description string   `xml:"description"`
			Meta        []struct {
				Name    string `xml:"name,attr"`
				Content string `xml:"content,attr"`
			} `xml:"meta"`
		} `xml:"metadata"`
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"manifest>item"`
	}
	if e = xml.Unmarshal(raw, &p); e != nil {
		return e
	}
	coverID := ""
	for _, m := range p.Metadata.Meta {
		if m.Name == "cover" {
			coverID = m.Content
		}
	}
	var cover []byte
	for _, it := range p.Items {
		if (coverID != "" && it.ID == coverID) || strings.Contains(" "+it.Properties+" ", " cover-image ") {
			cover, _ = read(path.Clean(path.Join(path.Dir(opfPath), it.Href)), 10<<20)
			break
		}
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, e := readMetadata()
	if e != nil {
		return e
	}
	idx := -1
	for i := range s.Books {
		if s.Books[i].FileName == target {
			idx = i
			break
		}
	}
	if idx < 0 {
		for i := range s.Books {
			if s.Books[i].FileName == filepath.Base(source) {
				idx = i
				s.Books[i].FileName = target
				break
			}
		}
		if idx < 0 {
			s.Books = append(s.Books, defaultMetadata(target))
			idx = len(s.Books) - 1
		}
	}
	b := &s.Books[idx]
	b.ImportType = "epub"
	b.OriginalName = filepath.Base(source)
	if b.Sources == nil {
		b.Sources = map[string]string{}
	}
	if v := strings.TrimSpace(p.Metadata.Title); v != "" && b.Sources["title"] != "user" {
		b.Title = v
		b.Sources["title"] = "embedded"
	}
	if len(p.Metadata.Creators) > 0 && b.Sources["authors"] != "user" {
		b.Authors = p.Metadata.Creators
		b.Sources["authors"] = "embedded"
	}
	if p.Metadata.Description != "" && b.Sources["description"] != "user" {
		b.Description = p.Metadata.Description
		b.Sources["description"] = "embedded"
	}
	if len(cover) > 0 && b.Sources["cover"] != "user" {
		if n, e := storeCover(cover); e == nil {
			b.Cover = n
			b.Sources["cover"] = "embedded"
		}
	}
	return writeMetadata(s)
}
