package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/constants"
	"github.com/tez-capital/ogun/notifications"
	"github.com/tez-capital/ogun/store"
	"github.com/trilitech/tzgo/tezos"
)

type Engine struct {
	ctx                context.Context
	collector          *rpcCollector
	store              *store.Store
	state              *state
	notificationConfig *notifications.DiscordNotificatorConfiguration
	logger             *slog.Logger
}

type EngineOptions struct {
	FetchAutomatically bool
	Transport          http.RoundTripper
}

var (
	DefaultEngineOptions = &EngineOptions{
		FetchAutomatically: true,
		Transport:          nil,
	}
	TestEngineOptions = &EngineOptions{
		FetchAutomatically: false,
	}
)

func NewEngine(ctx context.Context, config *configuration.Runtime, options *EngineOptions) (*Engine, error) {
	if options == nil {
		options = DefaultEngineOptions
	}

	collector, err := newRpcCollector(ctx, config.Providers, options.Transport)
	if err != nil {
		slog.Error("failed to create new RPC Collector", "error", err)
		return nil, err
	}

	store, err := store.NewStore(config)
	if err != nil {
		slog.Error("failed to create new store", "error", err)
		return nil, err
	}

	result := &Engine{
		ctx:                ctx,
		collector:          collector,
		store:              store,
		state:              newState(),
		notificationConfig: &config.DiscordNotificator,
		logger:             slog.Default(), // TODO: replace with custom logger
	}

	if options.FetchAutomatically {
		go result.fetchAutomatically()
	}

	return result, nil
}

func (e *Engine) fetchDelegateDelegationStateInternal(ctx context.Context, delegateAddress tezos.Address, cycle int64, options *FetchOptions) error {
	if options == nil {
		options = &defaultFetchOptions
	}

	if !options.Force && e.state.IsDelegateBeingFetched(cycle, delegateAddress) {
		slog.Debug("delegate delegation state is already being fetched", "cycle", cycle, "delegate", delegateAddress.String())
		return nil
	}

	if !options.Force {
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
		slog.Debug("failed to get delegate from", "cycle", cycle, "delegateAddress", delegateAddress, "error", err)
		return err
	}

	state, err := e.collector.GetDelegationState(ctx, delegate, cycle)
	var storableState *store.StoredDelegationState
	switch {
	case err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance:
		if options.Debug && errors.Is(err, constants.ErrMinimumDelegatedBalanceNotFound) {
			panic(err)
		}
		return err
	case err == constants.ErrDelegateHasNoMinimumDelegatedBalance:
		storableState = store.CreateStoredDelegationStateFromDelegationState(state)
		storableState.Status = store.DelegationStateStatusMinimumNotAvailable
	default:
		storableState = store.CreateStoredDelegationStateFromDelegationState(state)
	}

	return e.store.StoreDelegationState(storableState)
}

func (e *Engine) FetchDelegateDelegationState(ctx context.Context, delegateAddress tezos.Address, cycle int64, options *FetchOptions) error {
	slog.Info("fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "force_fetch", options)
	lastCompletedCycle, err := e.collector.GetLastCompletedCycle(ctx)
	if err != nil {
		slog.Error("failed to get last completed cycle", "error", err)
		return err
	}
	if cycle > lastCompletedCycle {
		return constants.ErrCycleDidNotEndYet
	}

	if err := e.fetchDelegateDelegationStateInternal(ctx, delegateAddress, cycle, options); err != nil {
		e.logger.Error("failed to fetch delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "error", err.Error())
		return err
	}
	slog.Info("finished fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String())
	return nil
}

func (e *Engine) FetchCycleDelegationStates(ctx context.Context, cycle int64, options *FetchOptions) error {
	slog.Info("fetching cycle delegation states", "cycle", cycle, "options", options)
	lastCompletedCycle, err := e.collector.GetLastCompletedCycle(ctx)
	if err != nil {
		e.logger.Error("failed to fetch last completed cycle number", "error", err.Error())
		return err
	}
	if cycle > lastCompletedCycle {
		e.logger.Error("cycle did not end yet", "cycle", cycle, "last_completed_cycle", lastCompletedCycle)
		return constants.ErrCycleDidNotEndYet
	}

	delegates, err := e.collector.GetActiveDelegatesFromCycle(ctx, cycle)
	if err != nil {
		e.logger.Error("failed to fetch active delegates from cycle", "cycle", cycle, "error", err.Error())
		return err
	}

	err = runInParallel(ctx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, item tezos.Address, mtx *sync.RWMutex) bool {
		err := e.fetchDelegateDelegationStateInternal(ctx, item, cycle, options)
		if err != nil {
			e.logger.Error("failed to fetch delegate delegation state", "cycle", cycle, "delegate", item.String(), "error", err.Error())
			msg := fmt.Sprintf("Failed to fetch delegate %s delegation state on cycle %d", item.String(), cycle)
			notifications.NotifyAdmin(e.notificationConfig, msg)
			return false
		}
		slog.Info("finished fetching delegate delegation state", "cycle", cycle, "delegate", item.String())
		return false
	})

	if err != nil {
		slog.Error("failed to fetch cycle", "cycle", cycle, "error", err.Error())
		return err
	}
	slog.Info("finished fetching cycle delegation states", "cycle", cycle)
	return nil
}

func (e *Engine) IsDelegateBeingFetched(cycle int64, delegate tezos.Address) bool {
	return e.state.IsDelegateBeingFetched(cycle, delegate)
}

func (e *Engine) GetDelegationState(ctx context.Context, delegate tezos.Address, cycle int64) (*store.StoredDelegationState, error) {
	cycle = e.collector.GetCycleBakingPowerOrigin(ctx, cycle)
	return e.store.GetDelegationState(delegate, cycle)
}

func (e *Engine) IsDelegationStateAvailable(ctx context.Context, delegate tezos.Address, cycle int64) (bool, error) {
	cycle = e.collector.GetCycleBakingPowerOrigin(ctx, cycle)
	return e.store.IsDelegationStateAvailable(delegate, cycle)
}

func (e *Engine) fetchAutomatically() {
	go func() {
		for {
			select {
			case <-e.ctx.Done():
				return
			default:
				time.Sleep(constants.OGUN_CYCLE_FETCH_FREQUENCY_MINUTES * time.Minute)

				lastCompletedCycle, err := e.collector.GetLastCompletedCycle(e.ctx)
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
					if err = e.FetchCycleDelegationStates(e.ctx, cycle, nil); err != nil {
						e.logger.Error("failed to fetch cycle delegation states", "cycle", cycle, "error", err.Error())
					}
					if err = e.store.PruneDelegationState(cycle); err != nil {
						e.logger.Error("failed to prune cycles out", "error", err.Error())
					}
				}

				e.state.SetLastOnChainCompletedCycle(lastCompletedCycle)
			}
		}
	}()
}
