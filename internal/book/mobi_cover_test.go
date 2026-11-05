package book

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedMOBICoverDoesNotReplaceUserCover(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	input := t.TempDir()
	source := filepath.Join(input, "book.txt")
	if err := os.WriteFile(source, []byte("正文"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLocal(source); err != nil {
		t.Fatal(err)
	}
	books, err := ListMetadata()
	if err != nil || len(books) != 1 {
		t.Fatalf("books: %#v %v", books, err)
	}
	image, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+nmWQAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	cover := filepath.Join(input, "cover.png")
	if err = os.WriteFile(cover, image, 0644); err != nil {
		t.Fatal(err)
	}
	if err = SetCover(books[0].ID, cover); err != nil {
		t.Fatal(err)
	}
	if err = CaptureMOBICover("book.txt", image); err != nil {
		t.Fatal(err)
	}
	books, err = ListMetadata()
	if err != nil || books[0].Sources["cover"] != "user" || !books[0].CoverChecked {
		t.Fatalf("cover: %#v %v", books, err)
	}
}
