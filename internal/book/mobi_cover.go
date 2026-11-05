package book

import "fmt"

func CaptureMOBICover(target string, data []byte) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	s, err := readMetadata()
	if err != nil {
		return err
	}
	for i := range s.Books {
		b := &s.Books[i]
		if b.FileName != target {
			continue
		}
		b.CoverChecked = true
		if b.Sources == nil {
			b.Sources = map[string]string{}
		}
		if len(data) > 0 && b.Sources["cover"] != "user" {
			cover, err := storeCover(data)
			if err != nil {
				return err
			}
			b.Cover = cover
			b.Sources["cover"] = "embedded"
		}
		return writeMetadata(s)
	}
	return fmt.Errorf("书籍信息不存在")
}
