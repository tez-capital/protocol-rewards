package core

import (
	"slices"
	"sync"

	"github.com/samber/lo"
	"github.com/trilitech/tzgo/tezos"
)

var (
	mtx sync.RWMutex
)

type state struct {
	lastFetchedCycle      int64
	delegatesBeingFetched map[int64][]tezos.Address
}

func newState() *state {
	return &state{
		delegatesBeingFetched: make(map[int64][]tezos.Address),
	}
}

func (s *state) AddDelegateBeingFetched(cycle int64, delegate ...tezos.Address) {
	mtx.Lock()
	defer mtx.Unlock()

	s.delegatesBeingFetched[cycle] = append(s.delegatesBeingFetched[cycle], delegate...)
}

func (s *state) RemoveCycleBeingFetched(cycle int64, delegate ...tezos.Address) {
	mtx.Lock()
	defer mtx.Unlock()

	if _, ok := s.delegatesBeingFetched[cycle]; !ok {
		return
	}

	s.delegatesBeingFetched[cycle] = lo.Filter(s.delegatesBeingFetched[cycle], func(d tezos.Address, _ int) bool {
		for _, del := range delegate {
			if d.Equal(del) {
				return false
			}
		}
		return true
	})
}

func (s *state) IsDelegateBeingFetched(cycle int64, delegate tezos.Address) bool {
	mtx.RLock()
	defer mtx.RUnlock()

	if _, ok := s.delegatesBeingFetched[cycle]; !ok {
		return false
	}

	return slices.Contains(s.delegatesBeingFetched[cycle], delegate)
}

func (s *state) SetLastFetchedCycle(cycle int64) {
	mtx.Lock()
	defer mtx.Unlock()

	s.lastFetchedCycle = cycle
}

func (s *state) GetLastOnChainCompletedCycle() int64 {
	mtx.RLock()
	defer mtx.RUnlock()

	return s.lastFetchedCycle
}
