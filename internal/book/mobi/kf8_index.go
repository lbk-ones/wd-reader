package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
)

// kf8IndexEntry 保存一个索引条目的名称和按标签定义解码的值，不将编号名称误当作目录标题。
type kf8IndexEntry struct {
	name string            // 骨架名称、分片插入位置或目录节点编号。
	tags map[byte][]uint32 // 标签对应的变长整数序列，含数量及位置等格式数据。
}

// kf8Index 为结构索引和原生目录共用的有界解码结果。
type kf8Index struct {
	entries []kf8IndexEntry   // 按索引记录顺序排列的条目。
	labels  map[uint32]string // 字符串表坐标到目录标题的映射。
}

// kf8Varint 读取高位在前、最高位标记结束的变长整数；每次读取必须在当前条目范围内完成。
func kf8Varint(data []byte, position *int) (uint32, error) {
	var value uint32
	for i := 0; i < 5 && *position < len(data); i++ {
		b := data[*position]
		*position += 1
		if value > 0x1ffffff {
			return 0, fmt.Errorf("索引变长整数溢出")
		}
		value = value<<7 | uint32(b&0x7f)
		if b&0x80 != 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("索引变长整数未完整结束")
}

// kf8Record 将相对当前格式头的记录编号转换为文件记录编号，兼容混合格式书籍。
func (r *Reader) kf8Record(id uint32) ([]byte, error) {
	base := max(0, r.FirstTextRecord-1)
	if uint64(id)+uint64(base) >= uint64(len(r.Offsets)) {
		return nil, fmt.Errorf("格式索引记录越界")
	}
	return r.LoadRecord(base + int(id))
}

// readKF8Index 读取根索引中的标签定义、后续数据记录和字符串表。
// 与旧的近似目录解析分离：所有偏移和数量都经过校验，任一损坏即放弃精确定位。
func (r *Reader) readKF8Index(id uint32) (kf8Index, error) {
	result := kf8Index{labels: map[uint32]string{}}
	root, err := r.kf8Record(id)
	if err != nil || len(root) < 56 || string(root[:4]) != "INDX" {
		return result, fmt.Errorf("缺少有效的格式索引")
	}
	u32 := binary.BigEndian.Uint32
	headerLen, recordCount, labelCount := int(u32(root[4:8])), u32(root[24:28]), u32(root[52:56])
	if headerLen < 56 || headerLen > len(root)-12 || string(root[headerLen:headerLen+4]) != "TAGX" ||
		uint64(id)+uint64(recordCount)+uint64(labelCount) >= uint64(len(r.Offsets)-max(0, r.FirstTextRecord-1)) {
		return result, fmt.Errorf("格式索引头或记录数量无效")
	}
	tagx := root[headerLen:]
	tagLen, controls := int(u32(tagx[4:8])), int(u32(tagx[8:12]))
	if tagLen < 12 || tagLen > len(tagx) || (tagLen-12)%4 != 0 || controls < 1 || controls > 8 {
		return result, fmt.Errorf("格式索引标签定义无效")
	}
	definitions := tagx[12:tagLen]
	for record := uint32(0); record < recordCount; record++ {
		data, err := r.kf8Record(id + 1 + record)
		if err != nil || len(data) < 28 || string(data[:4]) != "INDX" {
			return result, fmt.Errorf("格式索引数据记录损坏")
		}
		idxt, count := int(u32(data[20:24])), int(u32(data[24:28]))
		if idxt < 28 || idxt > len(data)-4 || string(data[idxt:idxt+4]) != "IDXT" || count > (len(data)-idxt-4)/2 {
			return result, fmt.Errorf("格式索引偏移表越界")
		}
		for i := 0; i < count; i++ {
			start := int(binary.BigEndian.Uint16(data[idxt+4+2*i:]))
			end := idxt
			if i+1 < count {
				end = int(binary.BigEndian.Uint16(data[idxt+6+2*i:]))
			}
			if start < 28 || start >= end || end > idxt {
				return result, fmt.Errorf("格式索引条目范围无效")
			}
			entry := data[start:end]
			nameEnd := 1 + int(entry[0])
			if nameEnd+controls > len(entry) {
				return result, fmt.Errorf("格式索引条目缺少控制字节")
			}
			tags, err := decodeKF8Tags(entry[nameEnd:], controls, definitions)
			if err != nil {
				return result, err
			}
			result.entries = append(result.entries, kf8IndexEntry{name: string(entry[1:nameEnd]), tags: tags})
		}
	}
	// 字符串坐标由记录序号和记录内字节位置组成，不是连续拼接后的偏移。
	for i := uint32(0); i < labelCount; i++ {
		data, err := r.kf8Record(id + recordCount + 1 + i)
		if err != nil || i > 0xffff {
			return result, fmt.Errorf("格式索引字符串记录无效")
		}
		for position := 0; position < len(data); {
			// 字符串记录可带对齐用的全零尾部，它不是未结束的变长长度字段。
			if data[position] == 0 && len(bytes.Trim(data[position:], "\x00")) == 0 {
				break
			}
			start := position
			length, err := kf8Varint(data, &position)
			if err != nil || uint64(length) > uint64(len(data)-position) {
				return result, fmt.Errorf("格式索引字符串越界")
			}
			label, err := decode(data[position:position+int(length)], r.MobiBook.Headers.MobiHeader.Encoding)
			if err != nil {
				return result, err
			}
			result.labels[i*0x10000+uint32(start)] = label
			position += int(length)
		}
	}
	return result, nil
}

// decodeKF8Tags 按位掩码解码条目控制字节，先读取各标签的数量声明，再读取标签值。
// 多位掩码全置位时，数量声明是后续值占用的字节数，而非值的个数。
func decodeKF8Tags(entry []byte, controls int, definitions []byte) (map[byte][]uint32, error) {
	if controls < 1 || controls > len(entry) || len(definitions)%4 != 0 {
		return nil, fmt.Errorf("格式索引标签输入不完整")
	}
	type pendingTag struct {
		tag   byte // 待解码的标签编号。
		count int  // 固定值数量；字节模式下保存字节数量。
		bytes bool // 是否按字节长度而非值个数读取。
	}
	position, control := controls, 0
	var pending []pendingTag
	for i := 0; i < len(definitions); i += 4 {
		tag, values, mask, end := definitions[i], definitions[i+1], definitions[i+2], definitions[i+3]
		if end&1 != 0 {
			control++
			continue
		}
		if control >= controls || mask == 0 {
			return nil, fmt.Errorf("格式索引控制字节定义越界")
		}
		value := entry[control] & mask
		item := pendingTag{tag: tag, count: int(value>>bits.TrailingZeros8(mask)) * int(values)}
		if value == mask && bits.OnesCount8(mask) > 1 {
			length, err := kf8Varint(entry, &position)
			if err != nil || uint64(length) > uint64(len(entry)-position) {
				return nil, fmt.Errorf("格式索引标签长度无效")
			}
			item.count, item.bytes = int(length), true
		}
		pending = append(pending, item)
	}
	result := map[byte][]uint32{}
	for _, item := range pending {
		end := position + item.count
		if item.count > len(entry)-position {
			return nil, fmt.Errorf("格式索引标签值越界")
		}
		for n := 0; (item.bytes && position < end) || (!item.bytes && n < item.count); n++ {
			value, err := kf8Varint(entry, &position)
			if err != nil || (item.bytes && position > end) {
				return nil, fmt.Errorf("格式索引标签值未完整结束")
			}
			result[item.tag] = append(result[item.tag], value)
		}
	}
	return result, nil
}
