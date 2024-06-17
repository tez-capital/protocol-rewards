package core

import (
	"github.com/tez-capital/ogun/common"
	"github.com/trilitech/tzgo/tezos"
)

var (
	defaultFetchOptions = FetchOptions{}
	DebugFetchOptions   = FetchOptions{Force: true, Debug: true}
)

type OgunBalanceUpdate struct {
	// rpc.BalanceUpdate
	Address tezos.Address `json:"address"`
	Amount  int64         `json:"amount,string"`

	Kind     string `json:"kind"`
	Category string `json:"category"`

	Operation     tezos.OpHash            `json:"operation"`
	Index         int                     `json:"index"`
	InternalIndex int                     `json:"internal_index"`
	Source        common.CreationInfoKind `json:"source"`

	Delegate tezos.Address `json:"delegate"`
}

type OgunBalanceUpdates []OgunBalanceUpdate

func (e OgunBalanceUpdates) Len() int {
	return len(e)
}

func (e OgunBalanceUpdates) Add(updates ...OgunBalanceUpdate) OgunBalanceUpdates {
	return append(e, updates...)
}

type FetchOptions struct {
	Force bool
	Debug bool
}
