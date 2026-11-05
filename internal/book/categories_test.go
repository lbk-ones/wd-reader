package book

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCategoryLifecyclePreservesBooks(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(metadataDir(), 0755); err != nil {
		t.Fatal(err)
	}
	// A library from before categories were introduced remains readable.
	data := []byte(`{"version":1,"books":[{"id":"book","fileName":"a.txt","title":"Original","favorite":true}]}`)
	if err := os.WriteFile(filepath.Join(metadataDir(), "metadata.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	categories, err := ListCategories()
	if err != nil || len(categories) != 15 {
		t.Fatalf("legacy categories: %v, %v", categories, err)
	}
	category, err := SaveCategory("", "  History  ")
	if err != nil || category.ID == "" || category.Name != "History" {
		t.Fatalf("new category: %+v, %v", category, err)
	}
	for _, invalid := range []string{" ", "history"} {
		if _, err := SaveCategory("", invalid); err == nil {
			t.Fatalf("accepted invalid or duplicate category %q", invalid)
		}
	}
	if err := UpdateMetadataWithCategory("book", "Edited", nil, "", "missing"); err == nil {
		t.Fatal("accepted a missing category")
	}
	if err := UpdateMetadataWithCategory("book", "Edited", nil, "", category.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveCategory(category.ID, "World history"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateMetadata("book", "Final", nil, "Description"); err != nil {
		t.Fatal(err)
	}
	stored, err := readMetadata()
	if err != nil || stored.Books[0].CategoryID != category.ID || !stored.Books[0].Favorite ||
		stored.Categories[len(categories)].Name != "World history" {
		t.Fatalf("category metadata did not persist: %+v, %v", stored, err)
	}
	if err := DeleteCategory(category.ID); err != nil {
		t.Fatal(err)
	}
	stored, err = readMetadata()
	if err != nil || len(stored.Categories) != len(categories) || len(stored.Books) != 1 ||
		stored.Books[0].CategoryID != "" || stored.Books[0].Title != "Final" || !stored.Books[0].Favorite {
		t.Fatalf("deleting category damaged book: %+v, %v", stored, err)
	}
}

func TestConcurrentCategoryCreationRejectsDuplicates(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	var workers sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		workers.Go(func() {
			_, err := SaveCategory("", "Shared")
			results <- err
		})
	}
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	categories, err := ListCategories()
	if err != nil || successes != 1 || len(categories) != 16 {
		t.Fatalf("duplicate categories: %+v, successes=%d, %v", categories, successes, err)
	}
}

func TestCategoryPresetsAreAddedOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	existing, err := SaveCategory("", "玄幻")
	if err != nil {
		t.Fatal(err)
	}
	categories, err := ListCategories()
	if err != nil || len(categories) != 15 || categories[0].ID != existing.ID {
		t.Fatalf("presets duplicated existing category: %+v, %v", categories, err)
	}
	if _, err := SaveCategory(existing.ID, "我的小说"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(categories[1].ID); err != nil {
		t.Fatal(err)
	}
	categories, err = ListCategories()
	if err != nil || len(categories) != 14 || categories[0].Name != "我的小说" {
		t.Fatalf("renamed or deleted presets were restored: %+v, %v", categories, err)
	}
	stored, err := readMetadata()
	if err != nil || !stored.CategoryPresetsAdded {
		t.Fatalf("preset initialization was not saved: %+v, %v", stored, err)
	}
}

func TestFinalCategoryPresets(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	categories, err := ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"玄幻", "武侠", "言情", "科幻", "脑洞", "历史", "悬疑", "恐怖", "同人", "克苏鲁", "无限流", "青春", "耽美", "另类", "出版"}
	if len(categories) != len(names) {
		t.Fatalf("unexpected presets: %+v", categories)
	}
	for i, name := range names {
		if categories[i].Name != name {
			t.Fatalf("preset %d: got %q, want %q", i, categories[i].Name, name)
		}
	}
}

func TestInitializedCategoriesRemainUserManaged(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	store := metadataStore{
		Version:              1,
		CategoryPresetsAdded: true,
		Categories:           []Category{{ID: "custom", Name: "我的分类"}},
		Books:                []BookMetadata{{ID: "book", CategoryID: "custom"}},
	}
	if err := writeMetadata(store); err != nil {
		t.Fatal(err)
	}
	categories, err := ListCategories()
	if err != nil || len(categories) != 1 || categories[0].ID != "custom" {
		t.Fatalf("initialized categories were changed: %+v, %v", categories, err)
	}
	stored, err := readMetadata()
	if err != nil || stored.Books[0].CategoryID != "custom" {
		t.Fatalf("book category was changed: %+v, %v", stored, err)
	}
}
func TestAssignUncategorizedBooksIsAtomicAndRetryable(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	store := metadataStore{
		Version:    1,
		Categories: []Category{{ID: "target", Name: "目标"}, {ID: "other", Name: "其他"}},
		Books: []BookMetadata{
			{ID: "a", Title: "A", Favorite: true},
			{ID: "b", Title: "B"},
			{ID: "c", Title: "C", CategoryID: "other"},
		},
	}
	if err := writeMetadata(store); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]string{{"a", "missing"}, {"a", "c"}, {}} {
		if err := AssignUncategorizedBooks("target", ids); err == nil {
			t.Fatalf("accepted invalid selection %v", ids)
		}
	}
	stored, err := readMetadata()
	if err != nil || stored.Books[0].CategoryID != "" || stored.Books[2].CategoryID != "other" {
		t.Fatalf("failed assignment partially changed metadata: %+v, %v", stored, err)
	}
	if err := AssignUncategorizedBooks("missing", []string{"a"}); err == nil {
		t.Fatal("accepted missing category")
	}
	for range 2 {
		if err := AssignUncategorizedBooks("target", []string{"a", "b", "a"}); err != nil {
			t.Fatal(err)
		}
	}
	stored, err = readMetadata()
	if err != nil || stored.Books[0].CategoryID != "target" || stored.Books[1].CategoryID != "target" ||
		stored.Books[2].CategoryID != "other" || !stored.Books[0].Favorite || stored.Books[0].Title != "A" {
		t.Fatalf("unexpected assignment result: %+v, %v", stored, err)
	}
	if err := RemoveBookFromCategory("c", "target"); err == nil {
		t.Fatal("removed a book from a different category")
	}
	for range 2 {
		if err := RemoveBookFromCategory("a", "target"); err != nil {
			t.Fatal(err)
		}
	}
	stored, err = readMetadata()
	if err != nil || len(stored.Books) != 3 || stored.Books[0].CategoryID != "" ||
		!stored.Books[0].Favorite || stored.Books[0].Title != "A" || stored.Books[1].CategoryID != "target" ||
		stored.Books[2].CategoryID != "other" {
		t.Fatalf("removing from category damaged metadata: %+v, %v", stored, err)
	}
}
