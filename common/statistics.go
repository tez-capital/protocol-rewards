package common

import "github.com/trilitech/tzgo/tezos"

type DelegateCycleStatistics struct {
	ExternalStaked    int64 `json:"external_staked"`
	OwnStaked         int64 `json:"own_staked"`
	ExternalDelegated int64 `json:"external_delegated"`
	OwnDelegated      int64 `json:"own_delegated"`
}

type CycleStatistics struct {
	Cycle     int64                                     `json:"cycle"`
	Delegates map[tezos.Address]DelegateCycleStatistics `json:"delegates"`
}
