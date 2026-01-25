package mobi

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"io"
	"os"
	"regexp"
	"strings"
)

type RecordOffset struct {
	Start uint32
	End   uint32
}

// Reader Mobi 电子书读取器结构体
type Reader struct {
	Data            []byte         // 文件原始数据
	Offsets         []RecordOffset // 记录偏移量表
	MobiBook        *MOBI_BOOK     // Mobi 书籍结构信息
	FirstTextRecord int            // 第一条文本记录的索引
	Mobi6Header     *MOBI_HEADER   // 原始 Mobi6 头部 (用于 Hybrid 文件回退)
}

// LoadRecord 读取指定索引的记录数据
// index: 记录索引，从 0 开始
func (r *Reader) LoadRecord(index int) ([]byte, error) {
	if index < 0 || index >= len(r.Offsets) {
		return nil, io.EOF
	}
	offset := r.Offsets[index]
	if int(offset.End) > len(r.Data) {
		return nil, fmt.Errorf("record end offset %d out of bounds %d", offset.End, len(r.Data))
	}
	return r.Data[offset.Start:offset.End], nil
}

// parseHeaders 解析文件头部信息，包括 PalmDOC, Mobi, EXTH 和 KF8 头部
func (r *Reader) parseHeaders(index int) (*MOBI_HEADER, *EXTH_HEADER, *PALMDOC_HEADER, *KF8_HEADER, error) {

	record, err := r.LoadRecord(index)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var palmdoc PALMDOC_HEADER
	err = parseStruct[PALMDOC_HEADER](record, PALMDOC_HEADER_OFFSET_MAP, &palmdoc)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var mobiHeader MOBI_HEADER
	err = parseStruct[MOBI_HEADER](record, MOBI_HEADER_OFFSET_MAP, &mobiHeader)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if mobiHeader.Magic != "MOBI" {
		return nil, nil, nil, nil, errors.New("error: missing mobi header!!")
	}

	titleLanguage(record, &mobiHeader)

	exth, err := getExth(record[mobiHeader.Length+16:], &mobiHeader)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if mobiHeader.Version >= 8 {
		var kf8header KF8_HEADER
		err = parseStruct[KF8_HEADER](record, KF8_HEADER_OFFSET_MAP, &kf8header)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return &mobiHeader, exth, &palmdoc, &kf8header, nil
	}

	return &mobiHeader, exth, &palmdoc, nil, nil
}

// parseOffsetsInternal 解析记录偏移量表
func (r *Reader) parseOffsetsInternal(numRecords int) {
	r.Offsets = make([]RecordOffset, 0, numRecords)
	for i := 0; i < numRecords; i++ {
		nextIndex := i + 1
		// 78 is PDB header length
		offsetPos := 78 + i*8
		if offsetPos+8 > len(r.Data) {
			break
		}

		dataSubArr := r.Data[offsetPos : offsetPos+8] // Read 8 bytes (current offset info)
		begin := getByteArrayValue(bytesOffset(dataSubArr, 0, 4), "uint32").(uint32)

		// To find the end, we look at the next record's start.
		// But for the last record?
		// The original code:
		// dataSubArr2 := all[78+nextIndex*8 : 78+(i+2)*8]
		// end := getByteArrayValue(bytesOffset(dataSubArr2, 0, 4), "uint32")
		// This implies there is always a "next" entry or a dummy entry at the end?
		// MOBI format usually has a dummy end record or we use file size?
		// The original code assumed nextIndex is valid.

		offsetPosNext := 78 + nextIndex*8
		if offsetPosNext+4 > len(r.Data) {
			// Last record goes to end of file? Or use what we have?
			// Default to end of file if no next record offset found
			// But standard PDB usually has records contiguous.
			// Let's assume the original code logic: it reads next offset.
			// If i is the last record, i+1 might point to something else or be out of bounds.
			// But wait, NumRecords includes all records.
			// The original code loop `i < OFFSET_LENGTH`.
			break
		}

		dataSubArr2 := r.Data[offsetPosNext : offsetPosNext+4]
		end := getByteArrayValue(dataSubArr2, "uint32").(uint32)

		r.Offsets = append(r.Offsets, RecordOffset{Start: begin, End: end})
	}
}

// parsePDBHeader 解析 PDB (Palm Database) 头部信息
func (r *Reader) parsePDBHeader() (*PDB_HEADER, error) {
	if len(r.Data) < 78 {
		return nil, errors.New("file too small for PDB header")
	}
	headerBytes := bytesOffset(r.Data, 0, 78)
	var pdb PDB_HEADER
	err := parseStruct[PDB_HEADER](headerBytes, PDB_HEADER_OFFSET_MAP, &pdb)
	if err != nil {
		fmt.Println(err)
	}
	return &pdb, err
}

// huffcdic 初始化 HUFF/CDIC 解压缩算法
// 返回一个解压缩函数
func (r *Reader) huffcdic(mobi *MOBI_HEADER) (func([]byte) []byte, error) {
	huffRecord, err := r.LoadRecord(int(mobi.Huffcdic))
	if err != nil {
		return nil, err
	}
	var huff HUFF_HEADER
	err = parseStruct[HUFF_HEADER](huffRecord, HUFF_HEADER_MAP, &huff)
	if err != nil {
		return nil, err
	}
	if huff.Magic != "HUFF" {
		return nil, errors.New("Invalid HUFF record")
	}

	// table1 is indexed by byte value
	table1 := make([][3]uint32, 256)
	for i := 0; i < 256; i++ {
		offset := int(huff.Offset1) + i*4
		if offset+4 > len(huffRecord) {
			break
		}
		x := getByteArrayValue(huffRecord[offset:offset+4], "uint32").(uint32)
		table1[i] = [3]uint32{x & 0b1000_0000, x & 0b1_1111, x >> 8}
	}

	// table2 is indexed by code length
	table2 := make([][2]uint32, 33)
	table2[0] = [2]uint32{0, 0}
	for i := 1; i <= 32; i++ {
		offset := int(huff.Offset2) + (i-1)*8
		if offset+8 > len(huffRecord) {
			break
		}
		table2[i][0] = getByteArrayValue(huffRecord[offset:offset+4], "uint32").(uint32)
		table2[i][1] = getByteArrayValue(huffRecord[offset+4:offset+8], "uint32").(uint32)
	}

	dictionary := make([][2][]byte, 0)
	for i := 1; i < int(mobi.NumHuffcdic); i++ {
		record, err := r.LoadRecord(int(mobi.Huffcdic) + i)
		if err != nil {
			return nil, err
		}
		var cdic CDIC_HEADER
		err = parseStruct[CDIC_HEADER](record, CDIC_HEADER_OFFSET_MAP, &cdic)
		if err != nil {
			return nil, err
		}
		if cdic.Magic != "CDIC" {
			return nil, errors.New("Invalid CDIC record")
		}
		// `numEntries` is the total number of dictionary data across CDIC records
		// so `n` here is the number of entries in *this* record
		n := 1 << cdic.CodeLength
		if int(cdic.NumEntries)-dictionarySize(dictionary) < n {
			n = int(cdic.NumEntries) - dictionarySize(dictionary)
		}
		if int(cdic.Length) > len(record) {
			continue
		}
		buffer := record[cdic.Length:]
		for i := 0; i < n; i++ {
			if i*2+2 > len(buffer) {
				break
			}
			offset := int(getByteArrayValue(buffer[i*2:i*2+2], "uint32").(uint32))
			if offset+2 > len(buffer) {
				break
			}
			x := getByteArrayValue(buffer[offset:offset+2], "uint32").(uint32)
			length := int(x & 0x7fff)
			decompressed := (x & 0x8000) != 0
			// 把 bool转为 byte
			de := byte(0)
			if decompressed {
				de = 1
			}
			if offset+2+length > len(buffer) {
				break
			}
			value := buffer[offset+2 : offset+2+length]
			dictionary = append(dictionary, [2][]byte{value, {de}})
		}
	}
	var decompress func([]byte) []byte
	// Recursive function requires declaring it first
	decompress = func(byteArray []byte) []byte {
		var output []byte
		bitLength := len(byteArray) * 8
		for i := 0; i < bitLength; {
			bits := read32Bits(byteArray, i)
			var found, codeLength, value uint32
			byteVal := byte(bits >> 24)
			table1Bytes := table1[byteVal]
			found = table1Bytes[0]
			codeLength = table1Bytes[1]
			value = table1Bytes[2]
			if found == 0 {
				for codeLength < 32 && bits>>(32-codeLength) < table2[codeLength][0] {
					codeLength++
				}
				if codeLength <= 32 {
					value = table2[codeLength][1]
				}
			}

			if codeLength == 0 {
				i++ // avoid infinite loop if codeLength is 0 (should not happen in valid huff)
				continue
			}

			i += int(codeLength)
			if i > bitLength {
				break
			}
			code := int(value) - int(bits>>(32-codeLength))

			if code < 0 || code >= len(dictionary) {
				// Invalid code
				break
			}

			var result, decompressed []byte
			dicValue := dictionary[code]
			result = dicValue[0]
			decompressed = dicValue[1]
			if decompressed[0] == 0 {
				// the result is itself compressed
				result = decompress(result)
				// cache the result for next time
				dictionary[code][0] = result
				dictionary[code][1] = []byte{1}
			}
			output = concatTypedArray(output, result)
		}
		return output
	}
	return decompress, nil
}

// setUp 初始化 Reader，设置解压缩函数和数据清理函数
func (r *Reader) setUp() error {
	book := r.MobiBook
	// setting decompress's two methods
	palmdocHeader := book.Headers.PalmdocHeader
	compression := palmdocHeader.Compression
	var decFunc DecompressFunc
	if compression == 1 {
		decFunc = func(i []byte) ([]byte, error) {
			return i, nil
		}
	} else if compression == 2 {
		decFunc = func(i []byte) ([]byte, error) {
			doc := decompressPalmDOC(i)
			return doc, nil
		}
	} else if compression == 17480 {
		fun, err := r.huffcdic(book.Headers.MobiHeader)
		if err != nil {
			return err
		}
		decFunc = func(i []byte) ([]byte, error) {
			i2 := fun(i)
			return i2, nil
		}
	} else {
		decFunc = func(i []byte) ([]byte, error) {
			return i, nil // Unknown compression
		}
	}

	book.Decompress = decFunc
	var removeTrailingEntriesFun RemoveTrailEntriesFunc
	removeTrailingEntriesFun = func(array []byte) []byte {
		return removeTrailingEntries(array, book.Headers.MobiHeader)
	}
	book.RemoveTrailEntries = removeTrailingEntriesFun

	return nil
}

// analysisHeaders 分析文件头，构建 MobiBook 结构，处理 Hybrid (Mobi6+KF8) 格式
func (r *Reader) analysisHeaders() error {
	var mobiBook MOBI_BOOK

	pdbHeader, err := r.parsePDBHeader()
	if err != nil {
		return err
	}

	r.parseOffsetsInternal(pdbHeader.NumRecords)

	mobiHeader, exthHeader, palmdocHeader, kf8Header, err := r.parseHeaders(0)
	if err != nil {
		return err
	}
	r.Mobi6Header = mobiHeader // 保存 Mobi6 Header 副本

	isKF8 := mobiHeader.Version >= 8
	mobiBook.IS_KF8 = isKF8
	r.FirstTextRecord = 1

	if !isKF8 {
		if exthHeader != nil && exthHeader.ExthRow != nil {
			boundary := exthHeader.ExthRow.Boundary
			if boundary > 0 && boundary < 0xffffffff {
				// KF8 is appended
				// We need to re-parse headers from the boundary
				mobiHeaderKF8, exthHeaderKF8, palmdocHeaderKF8, kf8HeaderKF8, errKF8 := r.parseHeaders(int(boundary))
				if errKF8 == nil {
					mobiHeader = mobiHeaderKF8
					exthHeader = exthHeaderKF8
					palmdocHeader = palmdocHeaderKF8
					kf8Header = kf8HeaderKF8
					mobiBook.IS_KF8 = true
					r.FirstTextRecord = int(boundary) + 1
				}
			}
		}
	}

	var headers HEADERS
	headers.PdbHeader = pdbHeader
	headers.PalmdocHeader = palmdocHeader
	headers.MobiHeader = mobiHeader
	headers.ExthHeader = exthHeader
	headers.Kf8Header = kf8Header
	mobiBook.Headers = &headers

	r.MobiBook = &mobiBook
	return nil
}

// ExtractHtml 提取电子书的原始 HTML 内容 (不移除标签)
func (r *Reader) ExtractHtml() (string, error) {
	if r.MobiBook == nil || r.MobiBook.Headers == nil {
		return "", errors.New("book not initialized")
	}

	numTextRecords := r.MobiBook.Headers.PalmdocHeader.NumTextRecords
	if numTextRecords == 0 {
		return "", nil
	}

	// Buffer to hold all text record data
	var buf bytes.Buffer

	// Text records start at 1. Record 0 is header.
	start := r.FirstTextRecord
	if start <= 0 {
		start = 1
	}

	for i := 0; i < int(numTextRecords); i++ {
		raw, err := r.LoadRecord(start + i)
		if err != nil {
			continue
		}

		// Remove trailing entries
		// Strategy:
		// 1. Calculate the 'potential' trailing size.
		// 2. If it's very large (e.g. > 100 bytes) AND the record is full-sized (4096),
		//    it's suspicious. Valid trailing entries are usually small (signatures, multibyte flags).
		//    But wait, some signatures can be long?
		//    Foliate's logic is just "remove".
		//
		// Let's try to detect if we are cutting off too much.

		rawLen := len(raw)
		data := r.MobiBook.RemoveTrailEntries(raw)
		removedCount := rawLen - len(data)

		// Heuristic: If we removed a lot of data (e.g. > 16 bytes) and the resulting data is suspiciously small
		// or if we suspect false positive.
		// For "很纯很暧昧.mobi", maybe the records are full but get cut?
		// Let's print debug info for now.
		if removedCount > 0 {
			//fmt.Printf("DEBUG: Record %d removed %d bytes (Original: %d -> New: %d)\n", start+i, removedCount, rawLen, len(data))
		}
		if removedCount > 8 {
			// fmt.Printf("DEBUG: Skipped removal for record %d (would remove %d/%d bytes)\n", start+i, removedCount, rawLen)
			data = raw
		}

		// Decompress
		decompressed, err := r.MobiBook.Decompress(data)
		if err != nil {
			continue
		}

		// Write to buffer
		buf.Write(decompressed)
	}

	// Decode the entire buffer
	text, err := decode(buf.Bytes(), r.MobiBook.Headers.MobiHeader.Encoding)
	if err != nil {
		// fallback
		text = buf.String()
	}
	idx := strings.LastIndex(text, ">")
	if idx == -1 {
		return text, nil
	} else {
		text = text[:idx]
	}
	return text, nil
}

// ExtractText 提取电子书的纯文本内容
// 包含解压、移除尾部数据、解码字符集和去除 HTML 标签的完整流程
func (r *Reader) ExtractText() (string, error) {
	text, err := r.ExtractHtml()
	if err != nil {
		return "", err
	}

	// Strip HTML
	cleanText := stripHTML(text)

	return cleanText, nil
}

// byteArrayToUint16 将字节数组转换为 uint16 (大端序)
func byteArrayToUint16(byteArray []byte) (uint16, error) {
	if len(byteArray) < 2 {
		return 0, fmt.Errorf("byte array length must be at least 2 for uint16 conversion")
	}
	var result uint16
	buffer := bytes.NewBuffer(byteArray[:2])
	err := binary.Read(buffer, binary.BigEndian, &result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// byteArrayToUint32 将字节数组转换为 uint32 (大端序)
func byteArrayToUint32(byteArray []byte) (uint32, error) {
	if len(byteArray) < 4 {
		return 0, fmt.Errorf("byte array length must be at least 4 for uint32 conversion")
	}
	var result uint32
	buffer := bytes.NewBuffer(byteArray[:4])
	err := binary.Read(buffer, binary.BigEndian, &result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// byteArrayToUint64 将字节数组转换为 uint64 (大端序)
func byteArrayToUint64(byteArray []byte) (uint64, error) {
	if len(byteArray) < 8 {
		return 0, fmt.Errorf("byte array length must be at least 8 for uint64 conversion")
	}
	var result uint64
	buffer := bytes.NewBuffer(byteArray[:8])
	err := binary.Read(buffer, binary.BigEndian, &result)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// bytesOffset 从字节数组中截取指定偏移和长度的片段
func bytesOffset(data []byte, offset int, bytes int) []byte {
	if offset < 0 || bytes < 0 || offset+bytes > len(data) {
		return nil
	}
	return data[offset : offset+bytes]
}

// getByteArrayValue 根据类型字符串将字节数组转换为对应的值 (uint16, uint32, uint64, string)
func getByteArrayValue(data []byte, btype string) interface{} {
	if btype == "uint16" {
		toUint16, _ := byteArrayToUint16(data)
		return toUint16
	} else if btype == "uint32" {
		toUint32, _ := byteArrayToUint32(data)
		return toUint32
	} else if btype == "uint64" {
		toUint64, _ := byteArrayToUint64(data)
		return toUint64
	} else if btype == "string" {
		str := string(data)
		// 处理空字符串，将 '\u0000' 替换为 ""
		str = strings.TrimRight(str, "\u0000")
		return str
	}
	return nil
}

// getStructOffsetHeader 根据结构体定义的偏移量从缓冲区中读取数据
func getStructOffsetHeader(buffer []byte, structstr []interface{}) interface{} {
	var re []int
	i2 := structstr[0]
	re = append(re, i2.(int))
	i3 := structstr[1]
	re = append(re, i3.(int))
	i4 := structstr[2]
	s := i4.(string)
	reTe := getByteArrayValue(bytesOffset(buffer, i2.(int), i3.(int)), s)
	return reTe
}

// parseStruct 通用结构体解析函数，根据 offsetMap 解析字节数组并映射到结构体
// bytes: 字节数组
// offsetMap: 字段名 -> [偏移, 长度, 类型] 的映射
func parseStruct[T any](bytes []byte, offsetMap map[string][]interface{}, obj *T) error {
	jsons := map[string]interface{}{}
	for key := range offsetMap {
		jsons[key] = getStructOffsetHeader(bytes, offsetMap[key])
	}
	marshal, err1 := json.Marshal(jsons)
	if err1 != nil {
		return err1
	}
	err2 := json.Unmarshal(marshal, &obj)
	if err2 != nil {
		return err2
	}
	return nil
}

// titleLanguage 解析并设置 Mobi 头部中的标题和语言信息
func titleLanguage(array16 []byte, mob *MOBI_HEADER) {
	offset := mob.TitleOffset
	titleLength := mob.TitleLength
	if offset > 0 && titleLength > 0 {
		titleT := bytesOffset(array16, int(offset), int(titleLength))
		mob.Title = titleT
		// fmt.Println("title is ", string(titleT))
	}
	language := mob.LocaleLanguage
	localeRegion := int(mob.LocaleRegion)

	if language > 0 {
		lang := MOBI_LANG[uint32(language)]
		langLength := len(lang)
		if localeRegion > 0 && langLength > 0 {
			l2 := localeRegion >> 2
			if l2 < langLength {
				if val, ok := lang[l2].(string); ok {
					mob.Language = val
				}
			} else {
				if val, ok := lang[0].(string); ok {
					mob.Language = val
				}
			}
		}
	}

}

// decode 根据指定的编码 (1252 或 65001/UTF-8) 将字节数组解码为字符串
func decode(bytesArray []byte, encoding uint32) (string, error) {
	if encoding == 65001 {
		return strings.Trim(string(bytesArray), "\u0000"), nil
	} else if encoding == 1252 {
		decoder := charmap.Windows1252.NewDecoder()
		all, err := io.ReadAll(transform.NewReader(bytes.NewReader(bytesArray), decoder))
		return strings.TrimRight(string(all), "\u0000"), err
	} else {
		// Fallback to UTF-8 if unknown, or just return as is
		return string(bytesArray), nil // errors.New("not support encode method" + strconv.Itoa(int(encoding)))
	}
}

// getExth 解析 EXTH (扩展) 头部信息，包含元数据如作者、ISBN 等
func getExth(row1Bytes []byte, mobiHeader *MOBI_HEADER) (*EXTH_HEADER, error) {
	exthFlag := mobiHeader.ExthFlag
	hasExthFlag := (exthFlag & 0x40) != 0
	if hasExthFlag {
		// fmt.Println("find exth flag")
		var exthHeader EXTH_HEADER
		if len(row1Bytes) < 12 {
			return nil, nil // Not enough data
		}
		headerBytes := bytesOffset(row1Bytes, 0, 12)
		err := parseStruct[EXTH_HEADER](headerBytes, EXTH_HEADER_OFFSET_MAP, &exthHeader)
		if err != nil {
			return nil, err
		}
		if exthHeader.Magic != "EXTH" {
			return nil, errors.New("missing exth header!!")
		}
		beginOffset := 12
		// parse rows
		count := exthHeader.Count
		jsonMap := map[string]interface{}{}
		var extRowItem EXTH_ROW
		for i := 0; i < count; i++ {
			if beginOffset+8 > len(row1Bytes) {
				break
			}
			offset := bytesOffset(row1Bytes, beginOffset, 4)
			nameCode := getByteArrayValue(offset, "uint32")
			i2 := bytesOffset(row1Bytes, beginOffset+4, 4)
			valueLength := getByteArrayValue(i2, "uint32")

			vl := int(valueLength.(uint32))
			if vl < 8 {
				beginOffset += vl
				continue
			}

			recordTypeValue := EXTH_RECORD_TYPE[nameCode.(uint32)]
			if recordTypeValue != nil {
				var name string
				var transferTo string
				var many bool
				rdtVl := len(recordTypeValue)
				if rdtVl >= 1 {
					name = recordTypeValue[0].(string)
					if rdtVl >= 2 {
						if v, ok := recordTypeValue[1].(string); ok {
							transferTo = v
						}
						if rdtVl >= 3 {
							if v, ok := recordTypeValue[2].(bool); ok {
								many = v
							}
						}
					}
				}
				var va interface{}
				// calc val
				if beginOffset+vl > len(row1Bytes) {
					break
				}
				valueOffsetBytes := bytesOffset(row1Bytes, beginOffset+8, vl-8)
				if transferTo == "uint32" {
					va = getByteArrayValue(valueOffsetBytes, "uint32")
				} else {
					s, err2 := decode(valueOffsetBytes, mobiHeader.Encoding)
					if err2 != nil {
						fmt.Println(err2)
					}
					va = s
				}
				if name != "" {
					exist := jsonMap[name]
					if exist != nil {
						if many == true {
							if arr, ok := exist.([]interface{}); ok {
								exist = append(arr, va)
								jsonMap[name] = exist
							} else {
								// Should not happen if logic is consistent
								jsonMap[name] = []interface{}{exist, va}
							}
						}
					} else {
						if many == true {
							jsonMap[name] = []interface{}{va}
						} else {
							jsonMap[name] = va
						}
					}
				}
			}
			beginOffset += vl
		}
		marshal, err := json.Marshal(jsonMap)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(marshal, &extRowItem)
		if err != nil {
			return nil, err
		}
		exthHeader.ExthRow = &extRowItem
		return &exthHeader, nil
	} else {
		// fmt.Println("no exth flag")
	}
	return nil, nil
}

// decompressPalmDOC PalmDOC 解压缩算法
// 使用 LZ77 变体进行解压
func decompressPalmDOC(array []byte) []byte {
	var output []byte
	i := 0
	for i < len(array) {
		byteVal := array[i]
		if byteVal == 0 {
			output = append(output, 0)
			i++
		} else if byteVal <= 8 {
			if i+int(byteVal) >= len(array) {
				break
			}
			for j := i + 1; j <= i+int(byteVal); j++ {
				output = append(output, array[j])
			}
			i += int(byteVal) + 1
		} else if byteVal <= 0b0111_1111 {
			output = append(output, byteVal)
			i++
		} else if byteVal <= 0b1011_1111 {
			if i+1 >= len(array) {
				break
			}
			bytesVal := uint16(byteVal)<<8 | uint16(array[i+1])
			i += 2
			distance := (bytesVal & 0b0011_1111_1111_1111) >> 3
			length := (bytesVal & 0b111) + 3

			// 检查 output 是否还有足够的空间
			// 如果 distance > len(output)，这对于 LZ77 来说是无效的，因为我们不能回溯到缓冲区开始之前
			// 但在某些流式实现中，可能引用之前的块。这里我们假设是独立的记录解压。
			if int(distance) > len(output) {
				// 尝试填充
				// 如果 distance 无效，说明数据流已经无法正确解析
				// 为了防止后续继续错误，我们应该终止解压？或者只是填充？
				// 很多时候 PalmDOC 的末尾会有几个无效字节
				// 我们选择填充空格并继续
				for k := 0; k < int(length); k++ {
					output = append(output, ' ')
				}
				continue
			}

			// startCopy := len(output) - int(distance) // 未使用

			for j := 0; j < int(length); j++ {
				// 每次迭代重新计算读取位置，以支持重叠复制（distance < length）
				// 重叠复制意味着我们要读取刚刚写入的字节
				readPos := len(output) - int(distance)
				// 再次检查以防万一
				if readPos < 0 || readPos >= len(output) {
					output = append(output, 0)
				} else {
					output = append(output, output[readPos])
				}
			}
		} else {
			output = append(output, 32, byteVal^0b1000_0000)
			i++
		}
	}
	return output
}

func concatTypedArray(output []byte, result []byte) []byte {
	return append(output, result...)
}

// dictionarySize 计算压缩字典的总大小
func dictionarySize(dictionary [][2][]byte) int {
	size := 0
	for _, entry := range dictionary {
		size += len(entry[0])
	}
	return size
}

// read32Bits 从字节数组中读取 32 位数据 (大端序)
func read32Bits(byteArray []byte, i int) uint32 {
	start := i / 8
	// We need 4 bytes to form a uint32, but we might be at the end of the array.
	// The original logic `if start+4 > len(byteArray) { return 0 }` is safe but might cut off last bits?
	// The bits are shifted.
	if start+4 > len(byteArray) {
		// handle edge case: pad with zeros
		buf := make([]byte, 4)
		copy(buf, byteArray[start:])
		return getByteArrayValue(buf, "uint32").(uint32) << (i % 8)
	}
	return getByteArrayValue(byteArray[start:start+4], "uint32").(uint32) << (i % 8)
}

// NewReader 创建一个新的 Mobi 阅读器实例
func NewReader(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	all, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	reader := &Reader{
		Data: all,
	}

	err = reader.analysisHeaders()
	if err != nil {
		return nil, err
	}

	err = reader.setUp()
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// countBitsSet 统计一个整数中置位（bit 为 1）的数量
func countBitsSet(x uint32) int {
	count := 0
	for ; x > 0; x >>= 1 {
		if x&1 == 1 {
			count++
		}
	}
	return count
}

// getVarLenFromEndFoliate implements the logic from foliate-js to read trailing entry size
// It scans the last 4 bytes (or fewer).
// It resets the accumulator whenever a high-bit byte is found (marking start of a VarInt).
// Returns the size.
func getVarLenFromEndFoliate(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	// Read up to last 4 bytes
	start := len(data) - 4
	if start < 0 {
		start = 0
	}
	tail := data[start:]

	num := 0
	for _, b := range tail {
		if b&0x80 != 0 {
			num = 0
		}
		num = (num << 7) | int(b&0x7F)
	}
	return num
}

// removeTrailingEntries 根据 trailingFlags 移除字节切片末尾的元素
// 每条文本记录的末尾可能包含“Trailing Entries”（用于存储签名、修正数据等）。
func removeTrailingEntries(array []byte, mobi *MOBI_HEADER) []byte {
	flags := mobi.TrailingFlags
	multibyte := flags & 1
	numTrailingEntries := countBitsSet(flags >> 1)

	for i := 0; i < numTrailingEntries; i++ {
		length := getVarLenFromEndFoliate(array)
		if length <= 0 {
			// If length is 0, it means we probably didn't find a valid marker or it's 0.
			// foliate-js would remove 0 bytes.
			// But if it's invalid, we might want to stop?
			// foliate-js loop continues. We continue.
		}
		totalToRemove := length
		if totalToRemove > len(array) {
			break
		}
		array = array[:len(array)-totalToRemove]
	}
	if multibyte != 0 {
		if len(array) > 0 {
			length := (array[len(array)-1] & 0b11) + 1
			if int(length) <= len(array) {
				array = array[:len(array)-int(length)]
			}
		}
	}
	return array
}

// stripHTML 移除字符串中的 HTML 标签，并将块级元素转换为换行符
// 同时会自动解码 HTML 实体
func stripHTML(input string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(input))
	var sb strings.Builder
	inIgnoredTag := false

	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				break
			}
			break
		}
		if tokenType == html.StartTagToken {
			tagName, _ := tokenizer.TagName()
			tag := strings.ToLower(string(tagName))
			if tag == "style" || tag == "script" || tag == "head" || tag == "title" || tag == "link" || tag == "img" {
				inIgnoredTag = true
			}
			if tag == "br" || tag == "p" || tag == "div" || tag == "tr" || strings.HasPrefix(tag, "h") {
				sb.WriteString("\n")
			}
		} else if tokenType == html.EndTagToken {
			tagName, _ := tokenizer.TagName()
			tag := strings.ToLower(string(tagName))
			if tag == "style" || tag == "script" || tag == "head" || tag == "title" || tag == "link" || tag == "img" {
				inIgnoredTag = false
			}
			if tag == "p" || tag == "div" || tag == "tr" || strings.HasPrefix(tag, "h") {
				sb.WriteString("\n")
			}
		} else if tokenType == html.TextToken {
			if !inIgnoredTag {
				text := string(tokenizer.Text())
				decoded := html.UnescapeString(text)
				sb.WriteString(decoded)
			}
		} else if tokenType == html.SelfClosingTagToken {
			tagName, _ := tokenizer.TagName()

			tag := strings.ToLower(string(tagName))
			if tag == "br" {
				sb.WriteString("\n")
			}
		}
	}
	s := sb.String()
	re := regexp.MustCompile(`\n+`)
	s = re.ReplaceAllString(s, "\n")
	return s
}
