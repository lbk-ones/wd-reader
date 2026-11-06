package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// KF8Section 是按骨架和分片重建后的单个正文文档，目录目标与正文始终使用同一字节坐标。
type KF8Section struct {
	HTML     string              // 已将分片插回骨架的完整文档，包含目录目标的不可见标记。
	Targets  map[string][]string // 标记标识到原生目录标题的映射，同位置可有多级标题。
	TOCStart string              // 原书导引明确指定的目录起始标记；用于只隐藏目录展示区域。
}

// kf8FragmentPosition 保存分片在重建文档中的位置，供目录的分片编号与偏移转换为标记。
type kf8FragmentPosition struct {
	section int // 分片所属的阅读顺序文档编号。
	start   int // 分片在重建文档中的开始字节位置。
	length  int // 原始分片长度，用于拒绝越界目录位置。
}

// kf8Marker 表示待注入的安全标签边界，所有位置均在注入前计算，避免多个标记互相扰动。
type kf8Marker struct {
	position int    // 文档中开始标签前的字节偏移。
	id       string // 注入标记的唯一标识。
}

// ExtractKF8 严格解码正文主流、重建骨架和分片，再将二进制原生目录映射到具体正文元素。
// 不支持或损坏的结构返回错误，由书库适配层回退到原有文本提取；不会保存或改写原书。
func (r *Reader) ExtractKF8() ([]KF8Section, error) {
	if r.MobiBook == nil || r.MobiBook.Headers == nil || r.MobiBook.Headers.Kf8Header == nil {
		return nil, fmt.Errorf("书籍没有 KF8 格式结构")
	}
	h := r.MobiBook.Headers.Kf8Header
	raw, err := r.extractRawText(true)
	if err != nil {
		return nil, err
	}
	// 只读取第一条正文流，后续样式等资源不能被当作章节内容。
	fdst, err := r.kf8Record(uint32(h.Fdst))
	if err != nil || len(fdst) < 20 || string(fdst[:4]) != "FDST" {
		return nil, fmt.Errorf("缺少有效的正文流索引")
	}
	count := int(binary.BigEndian.Uint32(fdst[8:12]))
	if count < 1 || count > (len(fdst)-12)/8 {
		return nil, fmt.Errorf("正文流索引数量无效")
	}
	start, end := int(binary.BigEndian.Uint32(fdst[12:16])), int(binary.BigEndian.Uint32(fdst[16:20]))
	if start != 0 || end < start || end > len(raw) {
		return nil, fmt.Errorf("正文流范围越界")
	}
	raw = raw[start:end]
	skeletons, err := r.readKF8Index(uint32(h.Skel))
	if err != nil {
		return nil, err
	}
	fragments, err := r.readKF8Index(uint32(h.Frag))
	if err != nil {
		return nil, err
	}
	if len(skeletons.entries) == 0 {
		return nil, fmt.Errorf("正文没有有效骨架")
	}
	sections, positions, err := rebuildKF8(raw, skeletons.entries, fragments.entries)
	if err != nil {
		return nil, err
	}
	markers := make([][]kf8Marker, len(sections))
	// 目录的位置必须全部可验证；不能让部分损坏的目录取代整本书的文本检测。
	navigation, navErr := r.readKF8Index(r.MobiBook.Headers.MobiHeader.Indx)
	if navErr == nil {
		for _, entry := range navigation.entries {
			pos, label := entry.tags[6], entry.tags[3]
			if len(pos) < 2 || len(label) == 0 {
				return nil, fmt.Errorf("原生目录缺少标题或分片位置")
			}
			title := strings.TrimSpace(html.UnescapeString(navigation.labels[label[0]]))
			section, marker, err := locateKF8Marker(sections, positions, pos[0], pos[1])
			if err != nil || title == "" {
				return nil, fmt.Errorf("原生目录条目不能定位到正文")
			}
			sections[section].Targets[marker.id] = append(sections[section].Targets[marker.id], title)
			markers[section] = append(markers[section], marker)
		}
	}
	// 导引目录用于排除书末目录展示文本，不据此删除同一文档内的真实正文或附录。
	if guide, err := r.readKF8Index(uint32(h.Guide)); err == nil {
		for _, entry := range guide.entries {
			pos := entry.tags[6]
			if strings.EqualFold(entry.name, "toc") && len(pos) >= 2 {
				section, marker, err := locateKF8Marker(sections, positions, pos[0], pos[1])
				if err == nil {
					sections[section].TOCStart = marker.id
					markers[section] = append(markers[section], marker)
				}
			}
		}
	}
	for i := range sections {
		// 位置排序去重后一次性注入，避免前一个标记改变后一个标记的坐标。
		sort.Slice(markers[i], func(a, b int) bool { return markers[i][a].position < markers[i][b].position })
		var output strings.Builder
		last := 0
		seen := map[string]bool{}
		for _, marker := range markers[i] {
			if seen[marker.id] {
				continue
			}
			seen[marker.id] = true
			output.WriteString(sections[i].HTML[last:marker.position])
			fmt.Fprintf(&output, `<span id="%s"></span>`, marker.id)
			last = marker.position
		}
		output.WriteString(sections[i].HTML[last:])
		sections[i].HTML, err = decode([]byte(output.String()), r.MobiBook.Headers.MobiHeader.Encoding)
		if err != nil {
			return nil, err
		}
	}
	return sections, nil
}

// rebuildKF8 将每个骨架对应的分片依次插入原书指定位置，保留文档顺序及分片到正文的映射。
func rebuildKF8(raw []byte, skeletons, fragments []kf8IndexEntry) ([]KF8Section, map[uint32]kf8FragmentPosition, error) {
	var sections []KF8Section
	positions := map[uint32]kf8FragmentPosition{}
	fragStart := 0
	totalSize := 0
	for section, skeleton := range skeletons {
		counts, span := skeleton.tags[1], skeleton.tags[6]
		if len(counts) < 1 || len(span) < 2 || uint64(counts[0]) > uint64(len(fragments)-fragStart) {
			return nil, nil, fmt.Errorf("骨架分片数量无效")
		}
		offset, length := int(span[0]), int(span[1])
		if offset > len(raw) || length > len(raw)-offset {
			return nil, nil, fmt.Errorf("骨架正文范围越界")
		}
		if length > (512<<20)-totalSize {
			return nil, nil, fmt.Errorf("重建正文超过解析限制")
		}
		body := bytes.Clone(raw[offset : offset+length])
		var current []uint32
		for _, fragment := range fragments[fragStart : fragStart+int(counts[0])] {
			insert, err := strconv.Atoi(fragment.name)
			fragSpan, ids := fragment.tags[6], fragment.tags[4]
			insert -= offset
			if err != nil || len(fragSpan) < 2 || len(ids) < 1 || insert < 0 || insert > len(body) {
				return nil, nil, fmt.Errorf("正文分片插入位置无效")
			}
			begin := uint64(offset) + uint64(length) + uint64(fragSpan[0])
			finish := begin + uint64(fragSpan[1])
			if finish > uint64(len(raw)) {
				return nil, nil, fmt.Errorf("正文分片范围越界")
			}
			if _, exists := positions[ids[0]]; exists {
				return nil, nil, fmt.Errorf("正文分片编号重复")
			}
			chunk := raw[int(begin):int(finish)]
			if len(chunk) > (512<<20)-totalSize-len(body) {
				return nil, nil, fmt.Errorf("重建正文超过解析限制")
			}
			// 后续分片可能插入前面，先平移已经记录的位置，再记录本次分片起点。
			for _, id := range current {
				p := positions[id]
				// 不在已映射分片内部继续猜测偏移；异常交叠结构整体回退，避免生成错位目录。
				if insert > p.start && insert < p.start+p.length {
					return nil, nil, fmt.Errorf("正文分片发生交叠插入")
				}
				if p.start >= insert {
					p.start += len(chunk)
					positions[id] = p
				}
			}
			next := make([]byte, 0, len(body)+len(chunk))
			next = append(next, body[:insert]...)
			next = append(next, chunk...)
			next = append(next, body[insert:]...)
			body = next
			positions[ids[0]] = kf8FragmentPosition{section: section, start: insert, length: len(chunk)}
			current = append(current, ids[0])
		}
		sections = append(sections, KF8Section{HTML: string(body), Targets: map[string][]string{}})
		totalSize += len(body)
		fragStart += int(counts[0])
	}
	if fragStart != len(fragments) {
		return nil, nil, fmt.Errorf("存在未归属骨架的正文分片")
	}
	return sections, positions, nil
}

// locateKF8Marker 将分片编号和分片内字节偏移定位到其后的首个有效开始标签。
// 只在该分片范围内寻找，不跨到下一分片猜测；注入点必须在完整标签前，避免破坏属性或多字节文字。
func locateKF8Marker(sections []KF8Section, positions map[uint32]kf8FragmentPosition, fid, offset uint32) (int, kf8Marker, error) {
	p, ok := positions[fid]
	if !ok || uint64(offset) >= uint64(p.length) {
		return 0, kf8Marker{}, fmt.Errorf("目录分片偏移越界")
	}
	begin, end := p.start, p.start+p.length
	if end > len(sections[p.section].HTML) {
		return 0, kf8Marker{}, fmt.Errorf("目录目标文档越界")
	}
	data := sections[p.section].HTML[begin:end]
	tokenizer := xhtml.NewTokenizer(strings.NewReader(data))
	consumed := 0
	for {
		kind := tokenizer.Next()
		start := consumed
		consumed += len(tokenizer.Raw())
		if kind == xhtml.ErrorToken {
			return 0, kf8Marker{}, fmt.Errorf("目录目标没有可定位的元素")
		}
		// 从完整分片开始分词，不能从属性或文字中间创建新分词器，否则可能误认属性中的尖括号。
		if start >= int(offset) && (kind == xhtml.StartTagToken || kind == xhtml.SelfClosingTagToken) {
			return p.section, kf8Marker{position: begin + start, id: fmt.Sprintf("wd-kf8-%d-%d", fid, offset)}, nil
		}
	}
}
