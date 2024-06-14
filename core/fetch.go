package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/store"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
	"gorm.io/gorm"
)

func getCollector(config *configuration.Runtime) (*DefaultRpcCollector, error) {
	tezosSubsystemConfiguration, err := config.GetTezosConfiguration()
	if err != nil {
		return nil, err
	}

	if len(tezosSubsystemConfiguration.Providers) == 0 {
		return nil, errors.New("no valid rpc available")
	}

	rpcUrl := "https://eu.rpc.tez.capital/"
	collector, err := InitDefaultRpcCollector(rpcUrl)
	if err != nil {
		slog.Error("failed to initialize collector", "error", err)
		return nil, err
	}

	return collector, nil
}

func FetchDelegateData(delegateAddress string, db *gorm.DB, config *configuration.Runtime) error {
	collector, err := getCollector(config)
	if err != nil {
		return err
	}

	lastCompletedCycle, err := collector.GetLastCompletedCycle(defaultCtx)
	if err != nil {
		slog.Error("failed to fetch last completed cycle number", "error", err)
		return err
	}

	delegate, err := collector.GetDelegateFromCycle(defaultCtx, lastCompletedCycle, tezos.MustParseAddress(delegateAddress))
	if err != nil {
		slog.Error("failed to fetch delegate", "error", err)
		return err
	}

	slog.Info("getting delegation state")
	state, err := collector.GetDelegationState(defaultCtx, delegate)
	if err != nil {
		slog.Error("failed to fetch delegation state", "error", err)
		return err
	}
	result, err := json.MarshalIndent(state, "", "\t")
	if err != nil {
		slog.Error("failed to marshal delegation state", "error", err)
		return err
	}
	fmt.Println(string(result))
	return nil
}

func FetchAllDelegatesFromCycle(cycle int64, config *configuration.Runtime) ([]*rpc.Delegate, error) {
	collector, err := getCollector(config)
	if err != nil {
		return nil, err
	}

	delegateList, err := collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	if err != nil {
		slog.Error("failed to fetch active delegates list from", "cycle", cycle, "error", err)
		return nil, err
	}

	numDelegates := len(delegateList)
	results := make([]*rpc.Delegate, numDelegates)
	errs := make([]error, numDelegates)
	var wg sync.WaitGroup

	sem := make(chan struct{}, config.BatchSize)

	slog.Info("fetching all delegates from", "cycle", cycle)
	for i, delegate := range delegateList {
		wg.Add(1)
		go func(i int, delegate tezos.Address) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			delegateDetails, err := collector.GetDelegateFromCycle(defaultCtx, cycle, delegate)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = delegateDetails
		}(i, delegate)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			slog.Error("failed to fetch delegate from cycle", "cycle", cycle, "error", err)
			return nil, err
		}
	}

	// for _, v := range results {
	// 	fmt.Println(*v)
	// }

	return results, nil
}

func FetchAllDelegatesStatesFromCycle(cycle int64, config *configuration.Runtime) ([]*store.DelegationState, error) {
	collector, err := getCollector(config)
	if err != nil {
		return nil, err
	}

	delegates, err := FetchAllDelegatesFromCycle(cycle, config)
	if err != nil {
		slog.Error("failed to fetch active delegates from", "cycle", cycle, "error", err)
		return nil, err
	}

	numDelegates := len(delegates)
	records := make([]*store.DelegationState, 0, numDelegates)
	errs := make([]error, numDelegates)
	var wg sync.WaitGroup
	var mu sync.Mutex

	sem := make(chan struct{}, config.BatchSize)

	slog.Info("fetching all delegates states from", "cycle", cycle)

	for i, delegate := range delegates {
		wg.Add(1)
		go func(i int, delegate *rpc.Delegate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			delegateState, err := collector.GetDelegationState(defaultCtx, delegate)
			if err != nil {
				errs[i] = err
				return
			}
			mu.Lock()
			records = append(records, delegateState)
			mu.Unlock()
		}(i, delegate)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	// for _, v := range records {
	// 	fmt.Println(*v)
	// }

	return records, nil
}
