package core

import (
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type BalanceUpdateSource string

const (
	BlockBalanceUpdateSource               BalanceUpdateSource = "block"
	TransactionMetadataBalanceUpdateSource BalanceUpdateSource = "transaction-metadata"
	TransactionContentsBalanceUpdateSource BalanceUpdateSource = "transaction-contents"
	InternalResultBalanceUpdateSource      BalanceUpdateSource = "internal-result"
)

type ExtendedBalanceUpdate struct {
	rpc.BalanceUpdate

	Operation     tezos.OpHash        `json:"operation"`
	Index         int64               `json:"index"`
	InternalIndex int64               `json:"internal_index"`
	Source        BalanceUpdateSource `json:"kind"`
}

type ExtendedBalanceUpdates []ExtendedBalanceUpdate

func (e ExtendedBalanceUpdates) Len() int {
	return len(e)
}

func (e ExtendedBalanceUpdates) Add(updates ...ExtendedBalanceUpdate) ExtendedBalanceUpdates {
	return append(e, updates...)
}

func (e ExtendedBalanceUpdates) AddBalanceUpdates(opHash tezos.OpHash, index int64, source BalanceUpdateSource, updates ...rpc.BalanceUpdate) ExtendedBalanceUpdates {
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

type DelegationState struct {
	Baker         tezos.Address       `json:"baker"`
	Cycle         int64               `json:"cycle"`
	Level         int64               `json:"level"`
	Operation     tezos.OpHash        `json:"operation"`
	Index         int64               `json:"transaction_index"`
	InternalIndex int64               `json:"internal_result_index"`
	Source        BalanceUpdateSource `json:"kind"`

	// incluides baker
	Balances     map[tezos.Address]tezos.Z `json:"balances"`
	TotalBalance tezos.Z                   `json:"total_balance"`
}
