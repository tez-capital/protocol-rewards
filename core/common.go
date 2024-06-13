package core

import (
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type DelegationState struct {
	*rpc.Delegate

	Operation        tezos.OpHash
	TransactionIndex int

	DelegatorBalances map[tezos.Address]tezos.Z `json:"delegatorBalances"`
}
