package mobi

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"io"
	"os"
	"strconv"
	"strings"
)

// author:bokun.li
// 这个东西太难搞了 要了我半条老命

var (
	OFFSET_ARRAR  = make([][]interface{}, 0)
	OFFSET_LENGTH = 0
	ALL_BYTES     = make([]byte, 0)
)

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

func bytesOffset(data []byte, offset int, bytes int) []byte {
	if offset < 0 || bytes < 0 || offset+bytes > len(data) {
		return nil
	}
	return data[offset : offset+bytes]
}

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

// bytes byte array
// offsetMap offset map
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

func titleLanguage(array16 []byte, mob *MOBI_HEADER) {
	offset := mob.TitleOffset
	titleLength := mob.TitleLength
	if offset > 0 && titleLength > 0 {
		titleT := bytesOffset(array16, int(offset), int(titleLength))
		mob.Title = titleT
		fmt.Println("title is ", string(titleT))
	}
	language := mob.LocaleLanguage
	localeRegion := int(mob.LocaleRegion)

	if language > 0 {
		lang := MOBI_LANG[uint32(language)]
		langLength := len(lang)
		if localeRegion > 0 && langLength > 0 {
			l2 := localeRegion >> 2
			if l2 < langLength {
				mob.Language = lang[l2].(string)
			} else {
				mob.Language = lang[0].(string)
			}
		}
	}

}

func decode(bytesArray []byte, encoding uint32) (string, error) {
	if encoding == 65001 {
		return strings.Trim(string(bytesArray), "\u0000"), nil
	} else if encoding == 1252 {
		decoder := charmap.Windows1252.NewDecoder()
		all, err := io.ReadAll(transform.NewReader(bytes.NewReader(bytesArray), decoder))
		return strings.TrimRight(string(all), "\u0000"), err
	} else {
		return "", errors.New("not support encode method" + strconv.Itoa(int(encoding)))
	}
}

// from 0 begin
func loadRecord(index int) ([]byte, error) {
	if len(OFFSET_ARRAR) >= index || index < 0 {
		return nil, io.EOF
	}
	pdbRow0Offset := OFFSET_ARRAR[index]
	pdbRow0ByteArray := bytesOffset(ALL_BYTES, int(pdbRow0Offset[0].(uint32)), int(pdbRow0Offset[1].(uint32))-int(pdbRow0Offset[0].(uint32)))
	return pdbRow0ByteArray, nil
}
func getHeaders(index int) (*MOBI_HEADER, *EXTH_HEADER, *PALMDOC_HEADER, *KF8_HEADER, error) {

	record, err := loadRecord(index)
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
		return nil, nil, nil, nil, errors.New("error: mission mobi header!!")
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

// get exth header
// all is row 1
func getExth(row1Bytes []byte, mobiHeader *MOBI_HEADER) (*EXTH_HEADER, error) {
	exthFlag := mobiHeader.ExthFlag
	hasExthFlag := (exthFlag & 0x40) != 0
	if hasExthFlag {
		fmt.Println("find exth flag")
		var exthHeader EXTH_HEADER
		headerBytes := bytesOffset(row1Bytes, 0, 12)
		err := parseStruct[EXTH_HEADER](headerBytes, EXTH_HEADER_OFFSET_MAP, &exthHeader)
		if err != nil {
			return nil, err
		}
		if exthHeader.Magic != "EXTH" {
			return nil, errors.New("misssion exth header!!")
		}
		beginOffset := 12
		// parse rows
		count := exthHeader.Count
		jsonMap := map[string]interface{}{}
		var extRowItem EXTH_ROW
		for i := 0; i < count; i++ {
			offset := bytesOffset(row1Bytes, beginOffset, 4)
			nameCode := getByteArrayValue(offset, "uint32")
			i2 := bytesOffset(row1Bytes, beginOffset+4, 4)
			valueLength := getByteArrayValue(i2, "uint32")

			recordTypeValue := EXTH_RECORD_TYPE[nameCode.(uint32)]
			if recordTypeValue != nil {
				var name string
				var transferTo string
				var many bool
				rdtVl := len(recordTypeValue)
				if rdtVl >= 1 {
					name = recordTypeValue[0].(string)
					if rdtVl >= 2 {
						transferTo = recordTypeValue[1].(string)
						if rdtVl >= 3 {
							many = recordTypeValue[2].(bool)
						}
					}
				}
				var va interface{}
				// calc val
				valueOffsetBytes := bytesOffset(row1Bytes, beginOffset+8, int(valueLength.(uint32))-8)
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
							exist = append(exist.([]interface{}), va)
							jsonMap[name] = exist
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
			beginOffset += int(valueLength.(uint32))
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
		fmt.Println("no exth flag")
	}
	return nil, nil
}

// from 78 begin calc offset array,every record info offset is 8
func offsets(all []byte) {
	for i := 0; i < OFFSET_LENGTH; i++ {
		nextIndex := i + 1
		dataSubArr := all[78+i*8 : 78+nextIndex*8]
		begin := getByteArrayValue(bytesOffset(dataSubArr, 0, 4), "uint32")

		dataSubArr2 := all[78+nextIndex*8 : 78+(i+2)*8]
		end := getByteArrayValue(bytesOffset(dataSubArr2, 0, 4), "uint32")
		OFFSET_ARRAR = append(OFFSET_ARRAR, []interface{}{begin, end})
	}
}

// get pdb info
func pdbInfo(all []byte, pdb *PDB_HEADER) error {
	headerBytes := bytesOffset(all, 0, 78)
	err := parseStruct[PDB_HEADER](headerBytes, PDB_HEADER_OFFSET_MAP, pdb)
	if err != nil {
		fmt.Println(err)
	}
	OFFSET_LENGTH = pdb.NumRecords
	return err
}

// byte decompress 1
func decompressPalmDOC(array []byte) []byte {
	var output []byte
	i := 0
	for i < len(array) {
		byteVal := array[i]
		if byteVal == 0 {
			output = append(output, 0)
		} else if byteVal <= 8 {
			for j := i + 1; j <= i+int(byteVal); j++ {
				output = append(output, array[j])
			}
			i += int(byteVal)
		} else if byteVal <= 0b0111_1111 {
			output = append(output, byteVal)
		} else if byteVal <= 0b1011_1111 {
			bytes := uint16(byteVal)<<8 | uint16(array[i+1])
			i++
			distance := (bytes & 0b0011_1111_1111_1111) >> 3
			length := (bytes & 0b111) + 3
			for j := 0; j < int(length); j++ {
				index := len(output) - int(distance)
				output = append(output, output[index])
			}
		} else {
			output = append(output, 32, byteVal^0b1000_0000)
		}
		i++
	}
	return output
}

func concatTypedArray(output []byte, result []byte) []byte {
	return append(output, result...)
}

// byte decompress 2
func huffcdic(mobi *MOBI_HEADER) (func([]byte) []byte, error) {
	huffRecord, err := loadRecord(int(mobi.Huffcdic))
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
		x := getByteArrayValue(huffRecord[offset:offset+4], "uint32").(uint32)
		table1[i] = [3]uint32{x & 0b1000_0000, x & 0b1_1111, x >> 8}
	}

	// table2 is indexed by code length
	table2 := make([][2]uint32, 33)
	table2[0] = [2]uint32{0, 0}
	for i := 1; i <= 32; i++ {
		offset := int(huff.Offset2) + (i-1)*8
		table2[i][0] = getByteArrayValue(huffRecord[offset:offset+4], "uint32").(uint32)
		table2[i][1] = getByteArrayValue(huffRecord[offset+4:offset+8], "uint32").(uint32)
	}

	dictionary := make([][2][]byte, 0)
	for i := 1; i < int(mobi.NumHuffcdic); i++ {
		record, err := loadRecord(int(mobi.Huffcdic) + i)
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
		buffer := record[cdic.Length:]
		for i := 0; i < n; i++ {
			offset := int(getByteArrayValue(buffer[i*2:i*2+2], "uint32").(uint32))
			x := getByteArrayValue(buffer[offset:offset+2], "uint32").(uint32)
			length := int(x & 0x7fff)
			decompressed := (x & 0x8000) != 0
			// 把 bool转为 byte
			de := 0
			if decompressed {
				de = 1
			}
			value := buffer[offset+2 : offset+2+length]
			dictionary = append(dictionary, [2][]byte{value, {byte(de)}})
		}
	}
	var decompress func([]byte) []byte
	decompress = func(byteArray []byte) []byte {
		var output []byte
		bitLength := len(byteArray) * 8
		for i := 0; i < bitLength; {
			bits := read32Bits(byteArray, i)
			var found, codeLength, value uint32
			table1Bytes := table1[byte(bits>>24)]
			found = table1Bytes[0]
			codeLength = table1Bytes[1]
			value = table1Bytes[2]
			if found == 0 {
				for bits>>(32-codeLength) < table2[codeLength][0] {
					codeLength++
				}
				value = table2[codeLength][1]
			}
			i += int(codeLength)
			if i > bitLength {
				break
			}
			code := int(value) - int(bits>>(32-codeLength))
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

func dictionarySize(dictionary [][2][]byte) int {
	size := 0
	for _, entry := range dictionary {
		size += len(entry[0])
	}
	return size
}

func read32Bits(byteArray []byte, i int) uint32 {
	start := i / 8
	if start+4 > len(byteArray) {
		return 0
	}
	return getByteArrayValue(byteArray[start:start+4], "uint32").(uint32)
}

func Open(path string) {
	file, err := os.Open(path)
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(file)
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("出现异常", err)
		}
	}()

	if err != nil {
		fmt.Println("打开文件失败:", err)
		return
	}

	all, _ := io.ReadAll(file)
	ALL_BYTES = all

	mobiBook, err2 := analysisHeaders()

	if err2 != nil {
		panic(err)
	}

	err2 = setUp(mobiBook)
	if err2 != nil {
		panic(err)
	}
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

// getVarLenFromEnd 从字节切片的末尾读取一个可变长度的数量
func getVarLenFromEnd(byteArray []byte) uint {
	value := uint(0)
	for i := len(byteArray) - 1; i >= len(byteArray)-4 && i >= 0; i-- {
		byteVal := uint(byteArray[i])
		// `byte & 0b1000_0000` 表示值的开始
		if byteVal&0b1000_0000 != 0 {
			value = 0
		}
		value = (value << 7) | (byteVal & 0b111_1111)
	}
	return value
}

// removeTrailingEntries 根据 trailingFlags 移除字节切片末尾的元素
func removeTrailingEntries(array []byte, mobi *MOBI_HEADER) []byte {
	flags := mobi.TrailingFlags
	multibyte := flags & 1
	numTrailingEntries := countBitsSet(flags >> 1)

	for i := 0; i < numTrailingEntries; i++ {
		length := getVarLenFromEnd(array)
		array = array[:len(array)-int(length)]
	}
	if multibyte != 0 {
		length := (array[len(array)-1] & 0b11) + 1
		array = array[:len(array)-int(length)]
	}
	return array
}

func setUp(book *MOBI_BOOK) error {
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
		decFunc = func(i []byte) ([]byte, error) {
			fun, err := huffcdic(book.Headers.MobiHeader)
			if err != nil {
				return nil, err
			}
			i2 := fun(i)
			return i2, nil
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

func analysisHeaders() (*MOBI_BOOK, error) {

	var mobiBook MOBI_BOOK

	var pdbHeader PDB_HEADER
	err := pdbInfo(ALL_BYTES, &pdbHeader)
	if err != nil {
		return nil, err
	}
	offsets(ALL_BYTES)

	mobiHeader, exthHeader, palmdocHeader, kf8Header, err := getHeaders(0)
	if err != nil {
		return nil, err
	}

	isKF8 := mobiHeader.Version >= 8
	mobiBook.IS_KF8 = isKF8
	if !isKF8 {
		boundary := exthHeader.ExthRow.Boundary
		if boundary < 0xffffffff {
			mobiHeader, exthHeader, palmdocHeader, kf8Header, err = getHeaders(int(boundary))
			mobiBook.IS_KF8 = true
			isKF8 = true
		}
	}

	var headers HEADERS
	headers.PdbHeader = &pdbHeader
	headers.PalmdocHeader = palmdocHeader
	headers.MobiHeader = mobiHeader
	headers.ExthHeader = exthHeader
	headers.Kf8Header = kf8Header
	mobiBook.Headers = &headers

	return &mobiBook, nil
}
