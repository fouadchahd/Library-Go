package account

import "gorm.io/gorm"

type Account struct {
	gorm.Model
	Username string `json:"username" gorm:"unique"`
	Password string `json:"password"`
	Meta     []Meta `json:"meta"`
}

type Meta struct {
	gorm.Model
	Key       string
	Value     string
	AccountID uint
}

func (account Account) IsValid() bool {
	return !(account.Username == "" || account.Password == "")
}
