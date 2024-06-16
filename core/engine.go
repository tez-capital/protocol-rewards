package core

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tez-capital/ogun/common"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/constants"
	"github.com/tez-capital/ogun/store"
	"github.com/trilitech/tzgo/tezos"
)

type Engine struct {
	ctx       context.Context
	collector *rpcCollector
	store     *store.Store
	state     *state
	logger    *slog.Logger
}

func NewEngine(ctx context.Context, config *configuration.Runtime) (*Engine, error) {
	collector, err := newRpcCollector(ctx, config.Providers, nil)
	if err != nil {
		return nil, err
	}

	store, err := store.NewStore(config)
	if err != nil {
		return nil, err
	}

	result := &Engine{
		ctx:       ctx,
		collector: collector,
		store:     store,
		state:     newState(),
		logger:    slog.Default(), // TODO: replace with custom logger
	}

	go result.fetchAutomatically()

	return result, nil
}

func (e *Engine) fetchDelegateDelegationStateInternal(ctx context.Context, delegateAddress tezos.Address, cycle int64, forceFetch bool) error {
	if !forceFetch && e.state.IsDelegateBeingFetched(cycle, delegateAddress) {
		slog.Debug("delegate delegation state is already being fetched", "cycle", cycle, "delegate", delegateAddress.String())
		return nil
	}

	if !forceFetch {
		_, err := e.store.GetDelegationState(delegateAddress, cycle)
		if err == nil { // already fetched
			slog.Debug("delegate delegation state already fetched", "cycle", cycle, "delegate", delegateAddress.String())
			return nil
		}
	}

	slog.Debug("fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String())
	e.state.AddDelegateBeingFetched(cycle, delegateAddress)
	defer e.state.RemoveCycleBeingFetched(cycle, delegateAddress)

	delegate, err := e.collector.GetDelegateFromCycle(ctx, cycle, delegateAddress)
	if err != nil {
		return err
	}

	state, err := e.collector.GetDelegationState(ctx, delegate)
	var storableState *store.StoredDelegationState
	switch {
	case err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance:
		return err
	case err == constants.ErrDelegateHasNoMinimumDelegatedBalance:
		storableState = store.CreateStoredDelegationStateFromDelegationState(common.NewDelegationState(delegate))
		storableState.Cycle = cycle
		storableState.Status = store.DelegationStateStatusMinimumNotAvailable
	default:
		storableState = store.CreateStoredDelegationStateFromDelegationState(state)
	}

	return e.store.StoreDelegationState(storableState)
}

func (e *Engine) FetchDelegateDelegationState(ctx context.Context, delegateAddress tezos.Address, cycle int64, forceFetch bool) error {
	slog.Info("fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "force_fetch", forceFetch)
	lastCompletedCycle, err := e.collector.GetLastCompletedCycle(defaultCtx)
	if err != nil {
		return err
	}
	if cycle > lastCompletedCycle {
		return constants.ErrCycleDidNotEndYet
	}

	if err := e.fetchDelegateDelegationStateInternal(ctx, delegateAddress, cycle, forceFetch); err != nil {
		e.logger.Error("failed to fetch delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "error", err.Error())
		return err
	}
	slog.Info("finished fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String())
	return nil
}

func (e *Engine) FetchCycleDelegationStates(ctx context.Context, cycle int64, forceFetch bool) error {
	slog.Info("fetching cycle delegation states", "cycle", cycle, "force_fetch", forceFetch)
	lastCompletedCycle, err := e.collector.GetLastCompletedCycle(defaultCtx)
	if err != nil {
		e.logger.Error("failed to fetch last completed cycle number", "error", err.Error())
		return err
	}
	if cycle > lastCompletedCycle {
		e.logger.Error("cycle did not end yet", "cycle", cycle, "last_completed_cycle", lastCompletedCycle)
		return constants.ErrCycleDidNotEndYet
	}

	delegates, err := e.collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	if err != nil {
		e.logger.Error("failed to fetch active delegates from cycle", "cycle", cycle, "error", err.Error())
		return err
	}

	err = runInBatches(ctx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, item tezos.Address, mtx *sync.RWMutex) bool {
		err := e.fetchDelegateDelegationStateInternal(ctx, item, cycle, forceFetch)
		if err != nil {
			// warn or error??
			e.logger.Warn("failed to fetch delegate delegation state", "cycle", cycle, "delegate", item.String(), "error", err.Error())
		}
		return false
	})
	slog.Info("finished fetching cycle delegation states", "cycle", cycle)
	return err
}

func (e *Engine) IsDelegateBeingFetched(cycle int64, delegate tezos.Address) bool {
	return e.state.IsDelegateBeingFetched(cycle, delegate)
}

func (e *Engine) GetDelegationState(delegate tezos.Address, cycle int64) (*store.StoredDelegationState, error) {
	return e.store.GetDelegationState(delegate, cycle)
}

func (e *Engine) GetLastFetchedCycle() (int64, error) {
	return e.store.GetLastFetchedCycle()
}

func (e *Engine) fetchAutomatically() {
	go func() {
		for {
			select {
			case <-e.ctx.Done():
				return
			default:
				time.Sleep(constants.OGUN_CYCLE_FETCH_FREQUENCY_MINUTES * time.Minute)

				lastCompletedCycle, err := e.collector.GetLastCompletedCycle(defaultCtx)
				if err != nil {
					e.logger.Error("failed to fetch last completed cycle number", "error", err.Error())
					return
				}

				if e.state.lastOnChainCompletedCycle == lastCompletedCycle {
					slog.Debug("no new cycle completed", "last_on_chain_completed_cycle", e.state.lastOnChainCompletedCycle, "last_completed_cycle", lastCompletedCycle)
					continue
				}

				if e.state.lastOnChainCompletedCycle == 0 {
					cycle, _ := e.store.GetLastFetchedCycle()
					switch cycle {
					case 0:
						e.state.SetLastOnChainCompletedCycle(lastCompletedCycle - 1)
					default:
						e.state.SetLastOnChainCompletedCycle(cycle)
					}
				}

				if e.state.lastOnChainCompletedCycle+1 <= lastCompletedCycle {
					e.logger.Info("fetching missing delegation states", "last_on_chain_completed_cycle", e.state.lastOnChainCompletedCycle, "last_completed_cycle", lastCompletedCycle)
				}

				for cycle := e.state.lastOnChainCompletedCycle + 1; cycle <= lastCompletedCycle; cycle++ {
					if err := e.FetchCycleDelegationStates(defaultCtx, cycle, false); err != nil {
						e.logger.Error("failed to fetch cycle delegation states", "cycle", cycle, "error", err.Error())
					}
				}

				e.state.SetLastOnChainCompletedCycle(lastCompletedCycle)
			}
		}
	}()
}
