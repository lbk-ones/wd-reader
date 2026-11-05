package book

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFavoritesPersistAcrossMetadataEdits(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(metadataDir(), 0755); err != nil {
		t.Fatal(err)
	}
	// Existing libraries have no favorite field.
	data := []byte(`{"version":1,"books":[{"id":"first","fileName":"a.txt","title":"A"},{"id":"second","fileName":"b.txt","title":"B"}]}`)
	if err := os.WriteFile(filepath.Join(metadataDir(), "metadata.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := SetFavorite("first", true); err != nil {
		t.Fatal(err)
	}
	if err := UpdateMetadata("first", "Edited title", []string{"Author"}, "Description"); err != nil {
		t.Fatal(err)
	}
	stored, err := readMetadata()
	if err != nil || len(stored.Books) != 2 || !stored.Books[0].Favorite || stored.Books[1].Favorite {
		t.Fatalf("unexpected favorites after editing: %+v, %v", stored, err)
	}
	if err := SetFavorite("unknown", true); err == nil {
		t.Fatal("missing book should be rejected")
	}
	if err := SetFavorite("first", false); err != nil {
		t.Fatal(err)
	}
	stored, err = readMetadata()
	if err != nil || stored.Books[0].Favorite || stored.Books[0].Title != "Edited title" {
		t.Fatalf("unexpected metadata after removing favorite: %+v, %v", stored, err)
	}
}
