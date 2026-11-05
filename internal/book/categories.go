package book

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func categoryExists(store metadataStore, id string) bool {
	if id == "" {
		return true
	}
	for _, category := range store.Categories {
		if category.ID == id {
			return true
		}
	}
	return false
}

func ListCategories() ([]Category, error) {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	store, err := readMetadata()
	if err != nil {
		return nil, err
	}
	if !store.CategoryPresetsAdded {
		for _, name := range []string{"玄幻", "武侠", "言情", "科幻", "脑洞", "历史", "悬疑", "恐怖", "同人", "克苏鲁", "无限流", "青春", "耽美", "另类", "出版"} {
			exists := false
			for _, category := range store.Categories {
				if category.Name == name {
					exists = true
					break
				}
			}
			if !exists {
				store.Categories = append(store.Categories, Category{ID: uuid.NewString(), Name: name})
			}
		}
		store.CategoryPresetsAdded = true
		if err := writeMetadata(store); err != nil {
			return nil, err
		}
	}
	if store.Categories == nil {
		store.Categories = []Category{}
	}
	return store.Categories, nil
}

func SaveCategory(id, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return Category{}, fmt.Errorf("分类名称应为 1–64 个字符")
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	store, err := readMetadata()
	if err != nil {
		return Category{}, err
	}
	for _, category := range store.Categories {
		if category.ID != id && strings.EqualFold(category.Name, name) {
			return Category{}, fmt.Errorf("分类名称已存在")
		}
	}
	if id == "" {
		category := Category{ID: uuid.NewString(), Name: name}
		store.Categories = append(store.Categories, category)
		return category, writeMetadata(store)
	}
	for i := range store.Categories {
		if store.Categories[i].ID == id {
			store.Categories[i].Name = name
			return store.Categories[i], writeMetadata(store)
		}
	}
	return Category{}, fmt.Errorf("分类不存在")
}

func DeleteCategory(id string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	store, err := readMetadata()
	if err != nil {
		return err
	}
	for i, category := range store.Categories {
		if category.ID == id {
			store.Categories = append(store.Categories[:i], store.Categories[i+1:]...)
			for j := range store.Books {
				if store.Books[j].CategoryID == id {
					store.Books[j].CategoryID = ""
				}
			}
			return writeMetadata(store)
		}
	}
	return fmt.Errorf("分类不存在")
}

// AssignUncategorizedBooks validates the whole selection before writing it.
// A retry may include books already assigned to the same category.
func AssignUncategorizedBooks(categoryID string, bookIDs []string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	store, err := readMetadata()
	if err != nil {
		return err
	}
	if categoryID == "" || !categoryExists(store, categoryID) {
		return fmt.Errorf("分类不存在")
	}
	if len(bookIDs) == 0 {
		return fmt.Errorf("请选择书籍")
	}
	pending := make(map[string]bool, len(bookIDs))
	for _, id := range bookIDs {
		pending[id] = true
	}
	indices := []int{}
	for i, book := range store.Books {
		if !pending[book.ID] {
			continue
		}
		if book.CategoryID != "" && book.CategoryID != categoryID {
			return fmt.Errorf("书籍分类已改变，请刷新后重试")
		}
		indices = append(indices, i)
		delete(pending, book.ID)
	}
	if len(pending) != 0 {
		return fmt.Errorf("书籍不存在")
	}
	for _, i := range indices {
		store.Books[i].CategoryID = categoryID
	}
	return writeMetadata(store)
}

func RemoveBookFromCategory(bookID, categoryID string) error {
	metadataMu.Lock()
	defer metadataMu.Unlock()
	store, err := readMetadata()
	if err != nil {
		return err
	}
	for i := range store.Books {
		if store.Books[i].ID != bookID {
			continue
		}
		if store.Books[i].CategoryID == "" {
			return nil
		}
		if store.Books[i].CategoryID != categoryID {
			return fmt.Errorf("书籍分类已改变，请刷新后重试")
		}
		store.Books[i].CategoryID = ""
		return writeMetadata(store)
	}
	return fmt.Errorf("书籍不存在")
}
