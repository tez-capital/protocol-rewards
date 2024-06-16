package core

import "sync"

var (
	mtx sync.RWMutex
)

type state struct {
	lastOnChainCompletedCycle int64
	cyclesBeingFetched        []int64
}

func (s *state) isCycleBeingFetched(cycle int64) bool {
	mtx.RLock()
	defer mtx.RUnlock()

	for _, c := range s.cyclesBeingFetched {
		if c == cycle {
			return true
		}
	}

	return false
}

func (s *state) addCycleBeingFetched(cycle int64) {
	mtx.Lock()
	defer mtx.Unlock()

	s.cyclesBeingFetched = append(s.cyclesBeingFetched, cycle)
}

func (s *state) removeCycleBeingFetched(cycle int64) {
	mtx.Lock()
	defer mtx.Unlock()

	for i, c := range s.cyclesBeingFetched {
		if c == cycle {
			s.cyclesBeingFetched = append(s.cyclesBeingFetched[:i], s.cyclesBeingFetched[i+1:]...)
			return
		}
	}
}

func (s *state) setLastOnChainCompletedCycle(cycle int64) {
	mtx.Lock()
	defer mtx.Unlock()

	s.lastOnChainCompletedCycle = cycle
}

func (s *state) getLastOnChainCompletedCycle() int64 {
	mtx.RLock()
	defer mtx.RUnlock()

	return s.lastOnChainCompletedCycle
}
