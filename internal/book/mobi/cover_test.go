package mobi

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

func TestEXTHCoverOffsetZeroAndCoverRecord(t *testing.T) {
	data := make([]byte, 24)
	copy(data, "EXTH")
	binary.BigEndian.PutUint32(data[4:], 24)
	binary.BigEndian.PutUint32(data[8:], 1)
	binary.BigEndian.PutUint32(data[12:], 201)
	binary.BigEndian.PutUint32(data[16:], 12)
	exth, err := getExth(data, &MOBI_HEADER{ExthFlag: 0x40})
	if err != nil || exth == nil || !exth.ExthRow.HasCoverOffset || exth.ExthRow.CoverOffset != 0 {
		t.Fatalf("EXTH: %#v %v", exth, err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nmWQAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	header := &MOBI_HEADER{ResourceStart: 1}
	reader := &Reader{Data: append([]byte("x"), png...), Offsets: []RecordOffset{{Start: 0, End: 1}, {Start: 1, End: uint32(len(png) + 1)}}, Mobi6Header: header, Mobi6EXTH: exth, MobiBook: &MOBI_BOOK{Headers: &HEADERS{MobiHeader: header, ExthHeader: exth}}}
	if got := reader.ExtractCover(); string(got) != string(png) {
		t.Fatal("cover record not extracted")
	}
	exth.ExthRow.HasCoverOffset = false
	if got := reader.ExtractCover(); got != nil {
		t.Fatal("image without cover marker treated as cover")
	}
	exth.ExthRow.HasCoverOffset = true
	exth.ExthRow.CoverOffset = 999
	if got := reader.ExtractCover(); got != nil {
		t.Fatal("invalid record returned")
	}
}
