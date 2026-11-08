package main

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/gen2brain/heic"
)

// Some covers use HEIC even when the site labels them as JPEG.
func convertCover(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[4:8]) != "ftyp" {
		return data, nil
	}
	switch string(data[8:12]) {
	case "heic", "heix", "hevc", "hevx", "msf1":
	default:
		return data, nil
	}
	config, err := heic.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 4096 || config.Height > 4096 {
		return nil, fmt.Errorf("HEIC 封面尺寸超出限制")
	}
	img, err := heic.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
