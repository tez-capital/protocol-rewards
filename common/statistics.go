package common

import "github.com/trilitech/tzgo/tezos"

type DelegateCycleStatistics struct {
	Staked    int64 `json:"staked"`
	Delegated int64 `json:"delegated"`
}

type CycleStatistics struct {
	Cycle     int64                                     `json:"cycle"`
	Delegates map[tezos.Address]DelegateCycleStatistics `json:"delegates"`
}
