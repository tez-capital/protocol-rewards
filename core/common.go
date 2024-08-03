package core

import (
	"github.com/tez-capital/protocol-rewards/common"
	"github.com/trilitech/tzgo/tezos"
)

var (
	defaultFetchOptions = FetchOptions{}
	DebugFetchOptions   = FetchOptions{Force: true, Debug: true}
	ForceFetchOptions   = FetchOptions{Force: true}
)

type PRBalanceUpdate struct {
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

type PRBalanceUpdates []PRBalanceUpdate

func (e PRBalanceUpdates) Len() int {
	return len(e)
}

func (e PRBalanceUpdates) Add(updates ...PRBalanceUpdate) PRBalanceUpdates {
	return append(e, updates...)
}

type FetchOptions struct {
	Force bool
	Debug bool
}
