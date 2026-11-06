package mobi

import (
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestKF8CenturyIndexes 验证真实样本的骨架、分片与目录索引可独立解码，不改写原书。
func TestKF8CenturyIndexes(t *testing.T) {
	filename := "../../../test/百年孤独.azw3"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Skip("未提供可选的真实书籍样本")
	}
	r, err := NewReader(filename)
	if err != nil {
		t.Fatal(err)
	}
	h := r.MobiBook.Headers
	sections, err := r.ExtractKF8()
	if err != nil {
		t.Fatal(err)
	}
	targets := 0
	for _, section := range sections {
		for _, titles := range section.Targets {
			targets += len(titles)
		}
	}
	if targets != 20 {
		t.Fatalf("原生章节数量=%d，期望20", targets)
	}
	// 同时验证索引解码结果，确保不是绕过分片位置而按目录标题做近似分章。
	for _, id := range []uint32{uint32(h.Kf8Header.Skel), uint32(h.Kf8Header.Frag), h.MobiHeader.Indx, uint32(h.Kf8Header.Guide)} {
		idx, err := r.readKF8Index(id)
		if err != nil {
			t.Fatalf("索引 %d: %v", id, err)
		}
		if len(idx.entries) == 0 {
			t.Fatalf("索引 %d 没有条目", id)
		}
	}
}

// TestKF8IndexValues 验证高位在前的变长整数、按值数量及按字节数量两种控制掩码。
func TestKF8IndexValues(t *testing.T) {
	position := 0
	if value, err := kf8Varint([]byte{1, 0x80}, &position); err != nil || value != 128 {
		t.Fatalf("变长整数=%d，错误=%v", value, err)
	}
	definitions := []byte{1, 1, 1, 0, 6, 2, 6, 0, 0, 0, 0, 1}
	// 控制字节使标签六进入字节计数模式，后续两字节对应两个值。
	tags, err := decodeKF8Tags([]byte{7, 0x82, 0x83, 0x84, 0x85}, 1, definitions)
	if err != nil || !reflect.DeepEqual(tags[1], []uint32{3}) || !reflect.DeepEqual(tags[6], []uint32{4, 5}) {
		t.Fatalf("标签=%v，错误=%v", tags, err)
	}
	for _, entry := range [][]byte{{7}, {7, 0x82, 0x83, 0x04}, {7, 0x81, 0x83, 0x04, 0x85}} {
		if _, err := decodeKF8Tags(entry, 1, definitions); err == nil {
			t.Fatalf("接受了不完整标签：%x", entry)
		}
	}
	for _, data := range [][]byte{{1}, {0x7f, 0x7f, 0x7f, 0x7f, 0xff}} {
		position = 0
		if _, err := kf8Varint(data, &position); err == nil {
			t.Fatalf("接受了未结束或溢出的整数：%x", data)
		}
	}
}

// TestKF8IndexRejectsInvalidOffsets 验证损坏的索引头和偏移表返回错误，不产生切片越界。
func TestKF8IndexRejectsInvalidOffsets(t *testing.T) {
	root := make([]byte, 208)
	copy(root, "INDX")
	binary.BigEndian.PutUint32(root[4:8], 192)
	binary.BigEndian.PutUint32(root[24:28], 1)
	copy(root[192:], "TAGX")
	binary.BigEndian.PutUint32(root[196:200], 16)
	binary.BigEndian.PutUint32(root[200:204], 1)
	copy(root[204:], []byte{1, 1, 1, 0})
	for _, offset := range []uint32{0, 27, 0xffffffff} {
		record := make([]byte, 32)
		copy(record, "INDX")
		binary.BigEndian.PutUint32(record[20:24], offset)
		binary.BigEndian.PutUint32(record[24:28], 1)
		data := append(append([]byte{}, root...), record...)
		r := &Reader{Data: data, Offsets: []RecordOffset{{0, uint32(len(root))}, {uint32(len(root)), uint32(len(data))}}, FirstTextRecord: 1}
		if _, err := r.readKF8Index(0); err == nil {
			t.Fatalf("接受了损坏偏移：%d", offset)
		}
	}
}

// TestKF8FragmentRebuildAndBounds 验证后插入的分片会正确平移前分片目录位置，并拒绝越界目标。
func TestKF8FragmentRebuildAndBounds(t *testing.T) {
	skeleton := "<body></body>"
	first, second := "<p>第一段。</p>", "<p>第二段。</p>"
	entries := []kf8IndexEntry{{tags: map[byte][]uint32{1: {2}, 6: {0, uint32(len(skeleton))}}}}
	fragments := []kf8IndexEntry{
		{name: "6", tags: map[byte][]uint32{4: {10}, 6: {0, uint32(len(first))}}},
		{name: "6", tags: map[byte][]uint32{4: {11}, 6: {uint32(len(first)), uint32(len(second))}}},
	}
	sections, positions, err := rebuildKF8([]byte(skeleton+first+second), entries, fragments)
	if err != nil || sections[0].HTML != "<body>"+second+first+"</body>" {
		t.Fatalf("重建错误：%v，%v", sections, err)
	}
	_, marker, err := locateKF8Marker(sections, positions, 10, 0)
	if err != nil || !strings.HasPrefix(sections[0].HTML[marker.position:], first) {
		t.Fatalf("目录定位错误：%+v，%v", marker, err)
	}
	if _, _, err := locateKF8Marker(sections, positions, 10, uint32(len(first))); err == nil {
		t.Fatal("接受了越界分片偏移")
	}
	fragments[1].tags[6][1] = 0xffffffff
	if _, _, err := rebuildKF8([]byte(skeleton+first+second), entries, fragments); err == nil {
		t.Fatal("接受了越界正文分片")
	}
}
