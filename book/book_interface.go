package book

type BookService interface {
	InsertBook(book *Book) (int, error)
	RemoveBook(book *Book) (bool, error)
	UpdateBook(book *Book) (bool, error)
	GetBookByID(ID int) *Book
	GetBooks() []*Book
}
