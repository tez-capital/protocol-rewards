package store

import "gorm.io/gorm"

type Delegate struct {
	gorm.Model
	Address    string `gorm:"unique;not null"`
	Delegators []Delegator
}

type Delegator struct {
	gorm.Model
	Address           string `gorm:"unique;not null"`
	DelegateID        uint
	DelegatedBalances []DelegatedBalance
}

type DelegatedBalance struct {
	gorm.Model
	Cycle       int   `gorm:"not null"`
	Balance     int64 `gorm:"not null"`
	DelegatorID uint
}
