package book

import "fmt"

type BookRepository struct {
}

func (p BookRepository) InsertBook(book *Book) (int, error) {
	fmt.Println("Insert book", book)
	return 2, nil
}

func (p BookRepository) GetBooks() []*Book {
	fmt.Println("Insert book")
	books := []*Book{&Book{
		Isbn:  2324324,
		Label: "Book No1",
	},
		&Book{
			Isbn:  232324324,
			Label: "Book No2",
		}}
	return books
}

func (p BookRepository) NumberOfSide() {
	fmt.Println("Pentagon has 5 sides")
}
