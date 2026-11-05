package desktop

import "wd-reader/internal/book"

func (a *App) ListCategories() ([]book.Category, error) {
	return book.ListCategories()
}

func (a *App) SaveCategory(id, name string) (book.Category, error) {
	return book.SaveCategory(id, name)
}

func (a *App) DeleteCategory(id string) error {
	return book.DeleteCategory(id)
}

func (a *App) UpdateBookDetails(id, title string, authors []string, description, categoryID string) error {
	return book.UpdateMetadataWithCategory(id, title, authors, description, categoryID)
}

func (a *App) AssignUncategorizedBooks(categoryID string, bookIDs []string) error {
	return book.AssignUncategorizedBooks(categoryID, bookIDs)
}

func (a *App) RemoveBookFromCategory(bookID, categoryID string) error {
	return book.RemoveBookFromCategory(bookID, categoryID)
}
