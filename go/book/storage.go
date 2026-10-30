package book

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"wd-reader/go/book/epub/EpubBook"
	"wd-reader/go/book/mobi"
	"wd-reader/go/constant"
	"wd-reader/go/server"
)

func ParsePath() string { return filepath.Join(GetAppPath(), constant.BOOK_PATH, "parse") }
func PurePath() string  { return filepath.Join(GetAppPath(), constant.BOOK_PATH, "pure") }

var storageMu sync.Mutex
var importMu sync.Mutex

func copyExclusive(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dest)
	}
	return err
}

// EnsureStorage moves legacy shelf files only after a copy exists in both destinations.
func EnsureStorage() error {
	storageMu.Lock()
	defer storageMu.Unlock()
	for _, dir := range []string{ParsePath(), PurePath()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	root := filepath.Join(GetAppPath(), constant.BOOK_PATH)
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".txt" && ext != ".epub" {
			continue
		}
		src := filepath.Join(root, name)
		pure := filepath.Join(PurePath(), name)
		if _, err := os.Stat(pure); os.IsNotExist(err) {
			if err = copyExclusive(src, pure); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if same, err := sameFileContents(src, pure); err != nil || !same {
			return fmt.Errorf("迁移冲突，原文件仍保留在 %s", src)
		}
		if ext == ".txt" {
			dest := filepath.Join(ParsePath(), name)
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				if err = copyExclusive(src, dest); err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if same, err := sameFileContents(src, dest); err != nil || !same {
				return fmt.Errorf("迁移冲突，原文件仍保留在 %s", src)
			}
			if info, err := os.Stat(src); err == nil {
				if err = os.Chtimes(dest, info.ModTime(), info.ModTime()); err != nil {
					return err
				}
			}
		} else {
			dest := filepath.Join(ParsePath(), strings.TrimSuffix(name, filepath.Ext(name))+".txt")
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				f, err := os.CreateTemp(ParsePath(), ".parse-*.txt")
				if err != nil {
					return err
				}
				temp := f.Name()
				f.Close()
				os.Remove(temp)
				if err = parseOriginal(pure, temp); err != nil {
					os.Remove(temp)
					return fmt.Errorf("迁移 %s: %w", name, err)
				}
				if err = CaptureEPUBMetadata(pure, filepath.Base(dest)); err != nil {
					os.Remove(temp)
					return err
				}
				if err = os.Rename(temp, dest); err != nil {
					os.Remove(temp)
					return err
				}
			} else if err != nil {
				return err
			} else {
				return fmt.Errorf("迁移冲突，原文件仍保留在 %s", src)
			}
		}
		if err := os.Remove(src); err != nil {
			return err
		}
	}
	return nil
}
func sameFileContents(a, b string) (bool, error) {
	x, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	y, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return string(x) == string(y), nil
}

func parseOriginal(source, dest string) error {
	ext := strings.ToLower(filepath.Ext(source))
	switch ext {
	case ".txt":
		return copyExclusive(source, dest)
	case ".epub":
		b, err := EpubBook.ParseEpub(source, dest)
		if err != nil {
			return err
		}
		defer b.ZipReader.Close()
		return b.WriteEpub()
	case ".mobi", ".azw", ".azw3":
		r, err := mobi.NewReader(source)
		if err != nil {
			return err
		}
		text, err := r.ExtractText()
		if err != nil {
			return err
		}
		if text == "" {
			return fmt.Errorf("未提取到正文")
		}
		return os.WriteFile(dest, append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), 0644)
	case ".pdf", ".doc", ".docx", ".html", ".htmlz":
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		response, err := server.PostOctetStream(map[string]string{"filename": filepath.Base(source)}, data)
		if err != nil {
			return err
		}
		if !response.Success {
			return fmt.Errorf("文件转换失败")
		}
		r, err := http.Get(response.Data.DownloadUrl + response.Data.FileUrl)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		if r.StatusCode != http.StatusOK {
			return fmt.Errorf("转换文件下载失败: %s", r.Status)
		}
		f, err := os.CreateTemp("", "wd-reader-convert-*.epub")
		if err != nil {
			return err
		}
		temp := f.Name()
		defer os.Remove(temp)
		_, err = io.Copy(f, r.Body)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		return parseOriginal(temp, dest)
	default:
		return fmt.Errorf("不支持的书籍格式: %s", ext)
	}
}

type LibraryOriginal struct {
	Name       string `json:"name"`
	ParsedName string `json:"parsedName"`
	InShelf    bool   `json:"inShelf"`
}

func ListOriginals() ([]LibraryOriginal, error) {
	if err := EnsureStorage(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(PurePath())
	if err != nil {
		return nil, err
	}
	list := []LibraryOriginal{}
	books, err := ListMetadata()
	if err != nil {
		return nil, err
	}
	associated := map[string]string{}
	for _, b := range books {
		if b.OriginalName != "" {
			associated[b.OriginalName] = b.FileName
		}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		parsed := strings.TrimSuffix(name, filepath.Ext(name)) + ".txt"
		if linked := associated[name]; linked != "" {
			parsed = linked
		}
		_, err := os.Stat(filepath.Join(ParsePath(), parsed))
		list = append(list, LibraryOriginal{Name: name, ParsedName: parsed, InShelf: err == nil})
	}
	return list, nil
}
func DeleteOriginal(name string) error {
	if !validBookName(name) {
		return fmt.Errorf("invalid book name")
	}
	if err := EnsureStorage(); err != nil {
		return err
	}
	return os.Remove(filepath.Join(PurePath(), name))
}

func ImportOriginal(name string, overwrite bool) (string, error) {
	importMu.Lock()
	defer importMu.Unlock()
	return importOriginal(name, overwrite)
}

func importOriginal(name string, overwrite bool) (string, error) {
	if !validBookName(name) {
		return "", fmt.Errorf("invalid book name")
	}
	if err := EnsureStorage(); err != nil {
		return "", err
	}
	source := filepath.Join(PurePath(), name)
	if info, err := os.Stat(source); err != nil {
		return "", err
	} else if !info.Mode().IsRegular() {
		return "", fmt.Errorf("invalid source")
	}
	parsed := strings.TrimSuffix(name, filepath.Ext(name)) + ".txt"
	books, err := ListMetadata()
	if err != nil {
		return "", err
	}
	for _, b := range books {
		if b.OriginalName == name {
			parsed = b.FileName
			break
		}
	}
	dest := filepath.Join(ParsePath(), parsed)
	if _, err := os.Stat(dest); err == nil && !overwrite {
		return "", fmt.Errorf("书架中已存在此书")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	f, err := os.CreateTemp(ParsePath(), ".parse-*.txt")
	if err != nil {
		return "", err
	}
	temp := f.Name()
	f.Close()
	os.Remove(temp)
	defer os.Remove(temp)
	if err = parseOriginal(source, temp); err != nil {
		return "", err
	}
	if err = os.Rename(temp, dest); err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(name), ".epub") {
		if err = CaptureEPUBMetadata(source, parsed); err != nil {
			return "", err
		}
	}
	if ext := strings.ToLower(filepath.Ext(name)); ext == ".mobi" || ext == ".azw" || ext == ".azw3" {
		if r, e := mobi.NewReader(source); e == nil {
			title, description := "", ""
			var authors []string
			if r.MobiBook != nil && r.MobiBook.Headers != nil && r.MobiBook.Headers.ExthHeader != nil && r.MobiBook.Headers.ExthHeader.ExthRow != nil {
				m := r.MobiBook.Headers.ExthHeader.ExthRow
				title, authors, description = m.Title, m.Creator, m.Description
			}
			if err = CaptureMOBIMetadata(parsed, title, authors, description); err != nil {
				return "", err
			}
			if err = CaptureMOBICover(parsed, r.ExtractCover()); err != nil {
				return "", err
			}
		}
	}
	if err = RecordImport(parsed, name); err != nil {
		return "", err
	}
	return parsed, nil
}

func ImportLocal(source string) (string, error) {
	importMu.Lock()
	defer importMu.Unlock()
	if err := EnsureStorage(); err != nil {
		return "", err
	}
	base := filepath.Base(source)
	if !validBookName(base) {
		return "", fmt.Errorf("invalid book name")
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	name := base
	for n := 2; ; n++ {
		_, a := os.Stat(filepath.Join(PurePath(), name))
		_, b := os.Stat(filepath.Join(ParsePath(), strings.TrimSuffix(name, filepath.Ext(name))+".txt"))
		usedStem := false
		entries, err := os.ReadDir(PurePath())
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), strings.TrimSuffix(name, filepath.Ext(name))) {
				usedStem = true
				break
			}
		}
		if os.IsNotExist(a) && os.IsNotExist(b) && !usedStem {
			break
		}
		name = fmt.Sprintf("%s (%d)%s", stem, n, ext)
	}
	if err := copyExclusive(source, filepath.Join(PurePath(), name)); err != nil {
		return "", err
	}
	return importOriginal(name, false)
}

func ImportFiles(paths []string) string {
	errors := map[string]string{}
	for _, source := range paths {
		if source == "" {
			continue
		}
		local := source
		var temp string
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			r, e := http.Get(source)
			if e != nil {
				errors[source] = e.Error()
				continue
			}
			if r.StatusCode != http.StatusOK {
				r.Body.Close()
				errors[source] = r.Status
				continue
			}
			name, e := getFileName(r, source)
			if e != nil {
				r.Body.Close()
				errors[source] = e.Error()
				continue
			}
			dir, e := os.MkdirTemp("", "wd-reader-download-")
			if e != nil {
				r.Body.Close()
				errors[source] = e.Error()
				continue
			}
			temp = dir
			local = filepath.Join(dir, filepath.Base(name))
			f, e := os.Create(local)
			if e == nil {
				_, e = io.Copy(f, r.Body)
				f.Close()
			}
			r.Body.Close()
			if e != nil {
				os.RemoveAll(dir)
				errors[source] = e.Error()
				continue
			}
		}
		_, err := ImportLocal(local)
		if temp != "" {
			os.RemoveAll(temp)
		}
		if err != nil {
			errors[source] = err.Error()
		}
	}
	b, _ := json.Marshal(errors)
	return string(b)
}
