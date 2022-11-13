package book

import (
	"errors"
	"gorm.io/gorm"
	"reflect"
	"strings"
	"time"
)

type Book struct {
	gorm.Model
	Isbn  int    `json:"isbn" gorm:"unique"`
	Label string `json:"label"`
}

type Borrow struct {
	gorm.Model
	TransactionId string
	BorrowDate    time.Time
	DueDate       *time.Time
	ReturnDate    *time.Time
	BorrowedBy    int
	BookId        int
}

type Status string

const (
	Error   = "error"
	Success = "success"
)

type ApiResponse struct {
	Data   any    `json:"data"`
	Status Status `json:"status"`
}

func (b *Book) IsValid() (bool, error) {
	if reflect.ValueOf(b.Isbn).Kind() != reflect.Int {
		return false, errors.New("invalid ISBN code provided")
	}
	if b.Label == "" && strings.TrimSpace(b.Label) == "" {
		return false, errors.New("invalid label provided")
	}
	return true, nil
}

func NewBook(label string, isbn int) *Book {
	b := &Book{
		Isbn:  isbn,
		Label: label,
	}
	return b
}

func (b *Book) TableName() string {
	return "book"
}
