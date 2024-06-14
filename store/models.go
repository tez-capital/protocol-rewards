package store

import "github.com/trilitech/tzgo/tezos"

type BalanceUpdateSource string

type DelegationState struct {
	ID            uint                `gorm:"primaryKey,autoIncrement;"`
	Baker         tezos.Address       `json:"baker" gorm:"index"`
	Cycle         int64               `json:"cycle" gorm:"index"`
	Level         int64               `json:"level" gorm:"index"`
	Operation     tezos.OpHash        `json:"operation" gorm:"index"`
	Index         int64               `json:"transaction_index" gorm:"index"`
	InternalIndex int64               `json:"internal_result_index" gorm:"index"`
	Source        BalanceUpdateSource `json:"kind" gorm:"index"`

	// includes balances
	Balances     map[tezos.Address]tezos.Z `json:"balances" gorm:"type:json"`
	TotalBalance tezos.Z                   `json:"total_balance" gorm:"index"`
}
