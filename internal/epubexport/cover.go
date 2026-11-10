package epubexport

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
)

// CoverFormat 根据文件内容确定封面的 EPUB 文件名和媒体类型。文件名为空表示格式不受支持。
func CoverFormat(cover []byte) (filename, mediaType string) {
	mediaType = http.DetectContentType(cover)
	switch mediaType {
	case "image/jpeg":
		filename = "cover.jpg"
	case "image/png":
		filename = "cover.png"
	case "image/gif":
		filename = "cover.gif"
	case "image/webp":
		filename = "cover.webp"
	}
	return filename, mediaType
}

// DefaultCover creates a placeholder PNG for books without a source cover.
func DefaultCover() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 480, 720))
	fill := func(rect image.Rectangle, c color.RGBA) {
		draw.Draw(img, rect, image.NewUniform(c), image.Point{}, draw.Src)
	}
	navy := color.RGBA{26, 42, 63, 255}
	ink := color.RGBA{53, 76, 98, 255}
	paper := color.RGBA{246, 237, 216, 255}
	gold := color.RGBA{213, 170, 103, 255}
	fill(img.Bounds(), navy)
	fill(image.Rect(30, 30, 450, 690), ink)
	fill(image.Rect(34, 34, 446, 686), navy)
	fill(image.Rect(110, 269, 235, 450), paper)
	fill(image.Rect(245, 269, 370, 450), paper)
	for y := 301; y <= 414; y += 26 {
		fill(image.Rect(130, y, 215, y+3), ink)
		fill(image.Rect(265, y, 350, y+3), ink)
	}
	fill(image.Rect(180, 525, 300, 529), gold)

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}
