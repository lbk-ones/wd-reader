package book

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"wd-reader/internal/book/mobi"
	"wd-reader/internal/chapterdetect"
	"wd-reader/internal/chapterdetect/source"
)

// 原生章节索引的磁盘适配流程：
// 导入时解码原书，得到正文和同坐标目录，以正文指纹保存索引后写入转换文本；
// 阅读时优先读取当前文本对应的索引，未命中则从关联原书恢复目录并尝试对齐旧文本；
// 目录缺失、错位或恢复失败时返回空目录，由独立检测模块完成文本分章。
// 索引置于书库元数据目录，因此资源导出导入会一并携带，无需按文件名额外迁移。

// chapterIndexLimit 限制单个目录索引的读写大小，避免异常目录无限增长。
const chapterIndexLimit = 8 << 20

// nativeMisses 缓存目录恢复失败的结果，避免每次切章都重新解压无可靠目录的原书。
// 键包含正文指纹、原文件路径、大小和修改时间，正文或原书变化后可重新尝试。
// 只缓存失败，不缓存目录成功结果；成功目录由磁盘索引复用。
var nativeMisses = struct {
	sync.Mutex                 // 保护失败缓存，允许不同阅读请求并发查询。
	keys       map[string]bool // 已确认本次正文和原书组合无法恢复目录的键集合。
}{keys: map[string]bool{}}

// nativeChapterIndexPath 根据受控的正文摘要生成索引路径，与书籍展示名和文件名解耦。
func nativeChapterIndexPath(fingerprint string) string {
	return filepath.Join(metadataDir(), "chapter-index", fingerprint+".json")
}

// saveNativeChapterIndex 保存通过校验的原生目录；无效或空目录不创建索引。
// 临时文件与目标索引放在同一目录，完整写入并同步后再重命名，避免读到半写入数据。
func saveNativeChapterIndex(lines []string, anchors []chapterdetect.Anchor) error {
	if !chapterdetect.ValidateAnchors(lines, anchors) {
		return nil
	}
	fingerprint := chapterdetect.Fingerprint(lines)
	// 版本和指纹必须与锚点一起保存，读取时同时验证格式兼容性及正文身份。
	data, err := json.Marshal(chapterdetect.NativeIndex{Version: chapterdetect.IndexVersion, Fingerprint: fingerprint, Anchors: anchors})
	if err != nil {
		return err
	}
	if len(data) > chapterIndexLimit {
		return fmt.Errorf("原生目录超过保存限制")
	}
	filename := nativeChapterIndexPath(fingerprint)
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".chapter-index-*.tmp")
	if err != nil {
		return err
	}
	// 任一路径退出均清理临时文件；成功重命名后原临时路径已不存在。
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filename)
}

// readNativeSource 将支持的原文件解码为统一的正文与目录结构，不写入书库。
// 书籍格式选择与底层解压留在适配层，独立检测模块无需依赖旧书库的格式实现。
func readNativeSource(filename string) (source.Document, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".epub":
		return source.ReadEPUB(filename)
	case ".mobi", ".azw", ".azw3":
		reader, err := mobi.NewReader(filename)
		if err != nil {
			return source.Document{}, err
		}
		// KF8 先重建独立文档，并将二进制目录转换为安全标记；失败时沿用原文本回退路径。
		if reader.MobiBook.Headers.Kf8Header != nil {
			if sections, err := reader.ExtractKF8(); err == nil {
				inputs := make([]source.HTMLSection, 0, len(sections))
				for _, section := range sections {
					inputs = append(inputs, source.HTMLSection{HTML: section.HTML, Targets: section.Targets, TOCStart: section.TOCStart})
				}
				if document, err := source.ReadHTMLSections(inputs); err == nil {
					return document, nil
				}
			}
		}
		raw, err := reader.ExtractHtml()
		if err != nil {
			return source.Document{}, err
		}
		// 只有原始 UTF-8 编码可沿用字节位置目录；其他编码转换后仅信任命名锚点。
		return source.ReadMOBIHTML(strings.NewReader(raw), reader.MobiBook.Headers.MobiHeader.Encoding == 65001)
	default:
		return source.Document{}, fmt.Errorf("此原文件没有可读取的原生目录")
	}
}

// writeNativeSource 先持久化目录，再写入转换后的 UTF-8 正文，供导入流程提交临时正文文件。
// 索引写入失败会中止本次转换；索引以正文指纹命名，因此正文提交失败不会污染其他内容的定位。
func writeNativeSource(filename string, document source.Document) error {
	if err := saveNativeChapterIndex(document.Lines, document.Native); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(document.Text()), 0644)
}

// nativeBookChapters 获取当前正文可安全使用的原生目录；无法恢复时返回空切片并允许文本检测。
// 顺序为：指纹索引、关联原书的直接定位、旧版正文的标题对齐；读取不会改写现有正文。
func nativeBookChapters(filename string, lines []string) []chapterdetect.Anchor {
	fingerprint := chapterdetect.Fingerprint(lines)
	// 索引大小、格式版本、正文指纹和锚点位置全部有效才可复用，正文变化自然使旧索引失效。
	if info, err := os.Stat(nativeChapterIndexPath(fingerprint)); err == nil && info.Size() <= chapterIndexLimit {
		if data, err := os.ReadFile(nativeChapterIndexPath(fingerprint)); err == nil {
			var index chapterdetect.NativeIndex
			if json.Unmarshal(data, &index) == nil && index.Version == chapterdetect.IndexVersion &&
				index.Fingerprint == fingerprint && chapterdetect.ValidateAnchors(lines, index.Anchors) {
				return index.Anchors
			}
		}
	}
	// 仅在访问元数据时持锁，耗时的文件解码放在锁外，避免阻塞其他书库元数据操作。
	metadataMu.Lock()
	store, err := readMetadata()
	metadataMu.Unlock()
	if err != nil {
		return nil
	}
	var original string
	for _, book := range store.Books {
		if book.FileName == filename && validBookName(book.OriginalName) {
			original = filepath.Join(PurePath(), book.OriginalName)
			break
		}
	}
	if original == "" {
		return nil
	}
	info, err := os.Stat(original)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	// 原书状态参与缓存键；替换或修复原文件后，即使正文没变，也不会一直复用旧失败结果。
	key := fmt.Sprintf("%s:%s:%d:%d", fingerprint, original, info.Size(), info.ModTime().UnixNano())
	nativeMisses.Lock()
	missed := nativeMisses.keys[key]
	nativeMisses.Unlock()
	if missed {
		return nil
	}
	document, err := readNativeSource(original)
	if err == nil && len(document.Native) > 0 {
		anchors := document.Native
		if chapterdetect.Fingerprint(document.Lines) != fingerprint {
			// 旧转换文本不套用新行号：独立标题按标题对齐，无标题章节按已定位的首段正文对齐。
			anchors = chapterdetect.AlignNativeLines(lines, document.Lines, anchors)
		}
		if chapterdetect.ValidateAnchors(lines, anchors) {
			// 阅读路径的索引保存为尽力而为；即使书库只读，本次请求仍可使用内存中恢复出的目录。
			_ = saveNativeChapterIndex(lines, anchors)
			return anchors
		}
	}
	nativeMisses.Lock()
	// 对失败缓存设置容量上限，避免长期阅读大量书籍后持续占用内存。
	if len(nativeMisses.keys) >= 64 {
		nativeMisses.keys = map[string]bool{}
	}
	nativeMisses.keys[key] = true
	nativeMisses.Unlock()
	return nil
}
