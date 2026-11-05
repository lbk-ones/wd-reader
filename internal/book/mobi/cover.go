package mobi

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// ExtractCover uses the explicitly marked EXTH cover record, including offset zero.
func (r *Reader) ExtractCover() []byte {
	if r == nil || r.MobiBook == nil || r.MobiBook.Headers == nil {
		return nil
	}
	h := r.MobiBook.Headers
	choices := []struct {
		header *MOBI_HEADER
		exth   *EXTH_HEADER
	}{{r.Mobi6Header, r.Mobi6EXTH}, {h.MobiHeader, h.ExthHeader}}
	for _, c := range choices {
		if c.header == nil || c.exth == nil || c.exth.ExthRow == nil || !c.exth.ExthRow.HasCoverOffset {
			continue
		}
		index := uint64(c.header.ResourceStart) + uint64(c.exth.ExthRow.CoverOffset)
		if index >= uint64(len(r.Offsets)) {
			continue
		}
		data, err := r.LoadRecord(int(index))
		if err != nil || len(data) > 10<<20 {
			continue
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			return data
		}
	}
	return nil
}
