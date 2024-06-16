package core

import (
	"context"
	"log/slog"
	"sync"

	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/constants"
	"github.com/tez-capital/ogun/store"
	"github.com/trilitech/tzgo/tezos"
)

type engine struct {
	collector *rpcCollector
	store     *store.Store
	state     *state
	logger    slog.Logger
}

func NewEngine(ctx context.Context, config *configuration.Runtime) (*engine, error) {
	collector, err := newRpcCollector(ctx, config.Providers, nil)
	if err != nil {
		return nil, err
	}

	store, err := store.NewStore(config.Database.Unwrap())
	if err != nil {
		return nil, err
	}

	result := &engine{
		collector: collector,
		store:     store,
		state:     &state{},
		logger:    *slog.Default(), // TODO: replace with custom logger
	}

	go result.fetchAutomatically()

	return result, nil
}

// TODO:
// [ ] fetch single delegation state on demand - delegate + cycle
// [ ] fetch all delegates from cycle - cycle
// [ ] automatic collector

func (e *engine) fetchDelegateDelegationStateInternal(ctx context.Context, delegateAddress tezos.Address, cycle int64) error {
	delegate, err := e.collector.GetDelegateFromCycle(ctx, cycle, delegateAddress)
	if err != nil {
		return err
	}

	state, err := e.collector.GetDelegationState(ctx, delegate)
	if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
		return err
	}

	storableState := store.CreateStoredDelegationStateFromDelegationState(state)
	if err == constants.ErrDelegateHasNoMinimumDelegatedBalance {
		storableState.Status = store.DelegationStateStatusMinimumNotAvailable
	}

	return e.store.StoreDelegationState(storableState)
}

func (e *engine) FetchDelegateDelegationState(ctx context.Context, delegateAddress tezos.Address, cycle int64) error {
	lastCompletedCycle, err := e.collector.GetLastCompletedCycle(defaultCtx)
	if err != nil {
		return err
	}
	if cycle > lastCompletedCycle {
		return constants.ErrCycleDidNotEndYet
	}

	if err := e.fetchDelegateDelegationStateInternal(ctx, delegateAddress, cycle); err != nil {
		e.logger.Error("failed to fetch delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "error", err.Error())
		return err
	}
	return nil
}

func (e *engine) FetchCycleDelegationStates(ctx context.Context, cycle int64) error {
	lastCompletedCycle, err := e.collector.GetLastCompletedCycle(defaultCtx)
	if err != nil {
		e.logger.Error("failed to fetch last completed cycle number", "error", err.Error())
		return err
	}
	if cycle > lastCompletedCycle {
		e.logger.Error("cycle did not end yet", "cycle", cycle, "last_completed_cycle", lastCompletedCycle)
		return constants.ErrCycleDidNotEndYet
	}

	e.state.addCycleBeingFetched(cycle)
	defer e.state.removeCycleBeingFetched(cycle)

	delegates, err := e.collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	if err != nil {
		e.logger.Error("failed to fetch active delegates from cycle", "cycle", cycle, "error", err.Error())
		return err
	}

	err = runInBatches(ctx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, item tezos.Address, mtx *sync.RWMutex) bool {
		err := e.fetchDelegateDelegationStateInternal(ctx, item, cycle)
		if err != nil {
			// warn or error??
			e.logger.Warn("failed to fetch delegate delegation state", "cycle", cycle, "delegate", item.String(), "error", err.Error())
		}
		return false
	})
	return err
}

func (e *engine) fetchAutomatically() {

}
