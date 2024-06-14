package core

import (
	"github.com/tez-capital/ogun/store"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

const (
	NoneBalanceUpdateSource                store.BalanceUpdateSource = "none"
	BlockBalanceUpdateSource               store.BalanceUpdateSource = "block"
	TransactionMetadataBalanceUpdateSource store.BalanceUpdateSource = "transaction-metadata"
	TransactionContentsBalanceUpdateSource store.BalanceUpdateSource = "transaction-contents"
	InternalResultBalanceUpdateSource      store.BalanceUpdateSource = "internal-result"
)

type ExtendedBalanceUpdate struct {
	rpc.BalanceUpdate

	Operation     tezos.OpHash              `json:"operation"`
	Index         int64                     `json:"index"`
	InternalIndex int64                     `json:"internal_index"`
	Source        store.BalanceUpdateSource `json:"kind"`
}

type ExtendedBalanceUpdates []ExtendedBalanceUpdate

func (e ExtendedBalanceUpdates) Len() int {
	return len(e)
}

func (e ExtendedBalanceUpdates) Add(updates ...ExtendedBalanceUpdate) ExtendedBalanceUpdates {
	return append(e, updates...)
}

func (e ExtendedBalanceUpdates) AddBalanceUpdates(opHash tezos.OpHash, index int64, source store.BalanceUpdateSource, updates ...rpc.BalanceUpdate) ExtendedBalanceUpdates {
	for _, update := range updates {
		e = append(e, ExtendedBalanceUpdate{
			BalanceUpdate: update,
			Operation:     opHash,
			Index:         index,
			Source:        source,
		})
	}
	return e
}

func (e ExtendedBalanceUpdates) AddInternalResultBalanceUpdates(opHash tezos.OpHash, index int64, internalIndex int64, updates ...rpc.BalanceUpdate) ExtendedBalanceUpdates {
	for _, update := range updates {
		e = append(e, ExtendedBalanceUpdate{
			BalanceUpdate: update,
			Operation:     opHash,
			Index:         index,
			Source:        InternalResultBalanceUpdateSource,
			InternalIndex: internalIndex,
		})
	}
	return e
}
