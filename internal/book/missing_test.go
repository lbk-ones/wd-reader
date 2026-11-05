package book

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAmbiguousRenamesCanBeLinkedWithoutLosingMetadata(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	input := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		p := filepath.Join(input, name)
		if err := os.WriteFile(p, []byte("same content"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := ImportLocal(p); err != nil {
			t.Fatal(err)
		}
	}
	books, err := ListMetadata()
	if err != nil || len(books) != 2 {
		t.Fatalf("books %#v %v", books, err)
	}
	var id string
	for _, b := range books {
		if b.FileName == "a.txt" {
			id = b.ID
		}
	}
	if err = UpdateMetadata(id, "自定义书名", []string{"作者"}, "简介"); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"a.txt", "new-a.txt"}, {"b.txt", "new-b.txt"}} {
		if err = os.Rename(filepath.Join(ParsePath(), pair[0]), filepath.Join(ParsePath(), pair[1])); err != nil {
			t.Fatal(err)
		}
	}
	missing, err := ListMissingBooks()
	if err != nil || len(missing) != 2 {
		t.Fatalf("missing %#v %v", missing, err)
	}
	var a MissingBook
	for _, b := range missing {
		if b.ID == id {
			a = b
		}
	}
	if len(a.Candidates) != 2 || !a.CanRestore {
		t.Fatalf("candidates %#v", a)
	}
	if err = RelinkMissingBook(id, "new-a.txt"); err != nil {
		t.Fatal(err)
	}
	books, err = ListMetadata()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range books {
		if b.ID == id {
			found = true
			if b.FileName != "new-a.txt" || b.Title != "自定义书名" || b.OriginalName != "a.txt" {
				t.Fatalf("linked %#v", b)
			}
		}
	}
	if !found {
		t.Fatal("missing linked record")
	}
	originals, err := ListOriginals()
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range originals {
		if o.Name == "a.txt" && (!o.InShelf || o.ParsedName != "new-a.txt") {
			t.Fatalf("original mapping %#v", o)
		}
	}
	if _, err = ImportOriginal("a.txt", false); err == nil {
		t.Fatal("reimport should detect linked shelf copy")
	}
}

func TestRelinkRejectsChangedFileAndCustomizedTarget(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	input := t.TempDir()
	p := filepath.Join(input, "a.txt")
	os.WriteFile(p, []byte("same"), 0644)
	if _, err := ImportLocal(p); err != nil {
		t.Fatal(err)
	}
	books, err := ListMetadata()
	if err != nil {
		t.Fatal(err)
	}
	id := books[0].ID
	if err = os.Rename(filepath.Join(ParsePath(), "a.txt"), filepath.Join(ParsePath(), "b.txt")); err != nil {
		t.Fatal(err)
	}
	// Add a second missing record with the same fingerprint so automatic matching remains ambiguous.
	metadataMu.Lock()
	s, err := readMetadata()
	if err != nil {
		metadataMu.Unlock()
		t.Fatal(err)
	}
	clone := s.Books[0]
	clone.ID = "other"
	clone.FileName = "other.txt"
	s.Books = append(s.Books, clone)
	err = writeMetadata(s)
	metadataMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ListMissingBooks(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(ParsePath(), "b.txt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = RelinkMissingBook(id, "b.txt"); err == nil {
		t.Fatal("changed file accepted")
	}
	if err = os.WriteFile(filepath.Join(ParsePath(), "b.txt"), []byte("same"), 0644); err != nil {
		t.Fatal(err)
	}
	books, err = ListMetadata()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range books {
		if b.FileName == "b.txt" {
			if err = UpdateMetadata(b.ID, "custom target", nil, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = RelinkMissingBook(id, "b.txt"); err == nil {
		t.Fatal("customized metadata overwritten")
	}
}

//func TestDeleteMissingRecordKeepsOriginal(t *testing.T) {
//	dir := t.TempDir()
//	t.Chdir(dir)
//	t.Setenv("HOME", dir)
//	input := t.TempDir()
//	p := filepath.Join(input, "a.txt")
//	if err := os.WriteFile(p, []byte("story"), 0644); err != nil {
//		t.Fatal(err)
//	}
//	if _, err := ImportLocal(p); err != nil {
//		t.Fatal(err)
//	}
//	books, err := ListMetadata()
//	if err != nil {
//		t.Fatal(err)
//	}
//	id := books[0].ID
//	if err = DeleteMissingBook(id); err == nil {
//		t.Fatal("present book metadata deleted")
//	}
//	if err = os.Remove(filepath.Join(ParsePath(), "a.txt")); err != nil {
//		t.Fatal(err)
//	}
//	missing, err := ListMissingBooks()
//	if err != nil || len(missing) != 1 || !missing[0].CanRestore {
//		t.Fatalf("missing %#v %v", missing, err)
//	}
//	if err = DeleteMissingBook(id); err != nil {
//		t.Fatal(err)
//	}
//	if _, err = os.Stat(filepath.Join(PurePath(), "a.txt")); err != nil {
//		t.Fatal(err)
//	}
//	missing, err = ListMissingBooks()
//	if err != nil || len(missing) != 0 {
//		t.Fatalf("missing %#v %v", missing, err)
//	}
//}

func TestDeleteMissingRecordKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	input := t.TempDir()
	p := filepath.Join(input, "a.txt")
	if err := os.WriteFile(p, []byte("story"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLocal(p); err != nil {
		t.Fatal(err)
	}
	books, err := ListMetadata()
	if err != nil {
		t.Fatal(err)
	}
	id := books[0].ID
	if err = DeleteMissingBook(id); err == nil {
		t.Fatal("present book metadata deleted")
	}
	if err = os.Remove(filepath.Join(ParsePath(), "a.txt")); err != nil {
		t.Fatal(err)
	}
	missing, err := ListMissingBooks()
	if err != nil || len(missing) != 1 || !missing[0].CanRestore {
		t.Fatalf("missing %#v %v", missing, err)
	}
	if err = DeleteMissingBook(id); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(PurePath(), "a.txt")); err != nil {
		t.Fatal(err)
	}
	missing, err = ListMissingBooks()
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing %#v %v", missing, err)
	}
}
