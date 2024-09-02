package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/tez-capital/protocol-rewards/common"
	"github.com/tez-capital/protocol-rewards/configuration"
	"github.com/tez-capital/protocol-rewards/constants"
	"github.com/tez-capital/protocol-rewards/notifications"
	"github.com/tez-capital/protocol-rewards/store"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type Engine struct {
	ctx         context.Context
	collector   *rpcCollector
	store       *store.Store
	state       *state
	notificator *notifications.DiscordNotificator
	delegates   []tezos.Address
	logger      *slog.Logger
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

	collector, err := newRpcCollector(ctx, config.Providers, config.TzktProviders, options.Transport)
	if err != nil {
		slog.Error("failed to create new RPC Collector", "error", err)
		return nil, err
	}

	store, err := store.NewStore(config)
	if err != nil {
		slog.Error("failed to create new store", "error", err)
		return nil, err
	}

	notificator, err := notifications.InitDiscordNotificator(&config.DiscordNotificator)
	if err != nil {
		slog.Warn("failed to initialize notificator", "error", err)
	}

	result := &Engine{
		ctx:         ctx,
		collector:   collector,
		store:       store,
		state:       newState(),
		notificator: notificator,
		delegates:   config.Delegates,
		logger:      slog.Default(), // TODO: replace with custom logger
	}

	if options.FetchAutomatically {
		go result.fetchAutomatically()
	}

	return result, nil
}

func (e *Engine) fetchDelegateDelegationStateInternal(ctx context.Context, delegateAddress tezos.Address, cycle, lastBlockInTheCycle int64, options *FetchOptions) error {
	if options == nil {
		options = &defaultFetchOptions
	}

	lastBlockInTheCycleId := rpc.BlockLevel(lastBlockInTheCycle)

	if !options.Force && e.state.IsDelegateBeingFetched(cycle, delegateAddress) {
		e.logger.Debug("delegate delegation state is already being fetched", "cycle", cycle, "delegate", delegateAddress.String())
		return nil
	}

	if !options.Force {
		_, err := e.store.GetDelegationState(delegateAddress, cycle)
		if err == nil { // already fetched
			e.logger.Debug("delegate delegation state already fetched", "cycle", cycle, "delegate", delegateAddress.String())
			return nil
		}
	}

	e.logger.Debug("fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String())
	e.state.AddDelegateBeingFetched(cycle, delegateAddress)
	defer e.state.RemoveCycleBeingFetched(cycle, delegateAddress)

	delegate, err := e.collector.GetDelegateFromCycle(ctx, lastBlockInTheCycleId, delegateAddress)
	if err != nil {
		e.logger.Debug("failed to get delegate from", "cycle", cycle, "delegateAddress", delegateAddress, "error", err)
		return err
	}

	state, err := e.collector.GetDelegationState(ctx, delegate, cycle, lastBlockInTheCycleId)
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
	e.logger.Debug("fetched delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "baking_power", state.GetBakingPower())

	return e.store.StoreDelegationState(storableState)
}

func (e *Engine) FetchDelegateDelegationState(ctx context.Context, delegateAddress tezos.Address, cycle, lastBlockInTheCycle int64, options *FetchOptions) error {
	e.logger.Info("fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "force_fetch", options)
	lastCompletedCycle, _, err := e.collector.GetLastCompletedCycle(ctx)
	if err != nil {
		e.logger.Error("failed to get last completed cycle", "error", err)
		return err
	}
	if cycle > lastCompletedCycle {
		return constants.ErrCycleDidNotEndYet
	}

	if lastBlockInTheCycle == 0 {
		lastBlockInTheCycle = e.collector.determineLastBlockOfCycle(cycle)
	}

	if err := e.fetchDelegateDelegationStateInternal(ctx, delegateAddress, cycle, lastBlockInTheCycle, options); err != nil {
		e.logger.Error("failed to fetch delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String(), "error", err.Error())
		return err
	}
	e.logger.Info("finished fetching delegate delegation state", "cycle", cycle, "delegate", delegateAddress.String())
	return nil
}

func (e *Engine) getDelegates(ctx context.Context, lastBlockInTheCycle int64) ([]tezos.Address, error) {
	delegates, err := e.collector.GetActiveDelegatesFromCycle(ctx, rpc.BlockLevel(lastBlockInTheCycle))
	if err != nil {
		e.logger.Error("failed to fetch active delegates from block", "block", lastBlockInTheCycle, "error", err.Error())
		return nil, err
	}

	if len(e.delegates) == 0 {
		return delegates, nil
	}

	delegates = lo.Filter(delegates, func(d tezos.Address, _ int) bool {
		return slices.Contains(e.delegates, d)
	})

	return delegates, nil
}

func (e *Engine) FetchCycleDelegationStates(ctx context.Context, cycle, lastBlockInTheCycle int64, options *FetchOptions) error {
	e.logger.Info("fetching cycle delegation states", "cycle", cycle, "options", options)
	lastCompletedCycle, _, err := e.collector.GetLastCompletedCycle(ctx)
	if err != nil {
		e.logger.Error("failed to fetch last completed cycle number", "error", err.Error())
		return err
	}
	if cycle > lastCompletedCycle {
		e.logger.Error("cycle did not end yet", "cycle", cycle, "last_completed_cycle", lastCompletedCycle)
		return constants.ErrCycleDidNotEndYet
	}

	if lastBlockInTheCycle == 0 {
		lastBlockInTheCycle = e.collector.determineLastBlockOfCycle(cycle)
	}

	delegates, err := e.getDelegates(ctx, lastBlockInTheCycle)
	if err != nil {
		return err
	}

	err = runInParallel(ctx, delegates, constants.DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, item tezos.Address, mtx *sync.RWMutex) bool {
		err := e.fetchDelegateDelegationStateInternal(ctx, item, cycle, lastBlockInTheCycle, options)
		if err != nil {
			e.logger.Error("failed to fetch delegate delegation state", "cycle", cycle, "delegate", item.String(), "error", err.Error())
			msg := fmt.Sprintf("Failed to fetch delegate %s delegation state on cycle %d", item.String(), cycle)
			notifications.Notify(e.notificator, msg)
			return false
		}
		e.logger.Info("finished fetching delegate delegation state", "cycle", cycle, "delegate", item.String())
		return false
	})

	if err != nil {
		e.logger.Error("failed to fetch cycle", "cycle", cycle, "error", err.Error())
		return err
	}
	e.logger.Info("finished fetching cycle delegation states", "cycle", cycle)
	notifications.Notify(e.notificator, fmt.Sprintf("Finished fetching cycle %d delegation states", cycle))
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

func (e *Engine) Statisticts(ctx context.Context, cycle int64) (*common.CycleStatistics, error) {
	return e.store.Statistics(cycle)
}

func (e *Engine) fetchAutomatically() {
	go func() {
		for {
			select {
			case <-e.ctx.Done():
				return
			default:
				time.Sleep(constants.CYCLE_FETCH_FREQUENCY_MINUTES * time.Minute)

				lastOnChainCompletedCycle, lastBlockInTheCycle, err := e.collector.GetLastCompletedCycle(e.ctx)
				if err != nil {
					e.logger.Error("failed to fetch last completed cycle number", "error", err.Error())
					return
				}

				lastFetchedCycle := e.state.GetLastFetchedCycle()
				if lastFetchedCycle >= lastOnChainCompletedCycle {
					e.logger.Debug("no new cycle completed", "last_fetched_cycle", lastFetchedCycle, "last_on_chain_completed_cycle", lastOnChainCompletedCycle)
					continue
				}

				if lastFetchedCycle == 0 {
					cycle, _ := e.store.GetLastFetchedCycle()
					switch cycle {
					case 0:
						e.state.SetLastFetchedCycle(lastOnChainCompletedCycle - 1)
					default:
						e.state.SetLastFetchedCycle(cycle)
					}
					lastFetchedCycle = e.state.GetLastFetchedCycle()
				}

				if lastFetchedCycle+1 <= lastOnChainCompletedCycle {
					e.logger.Info("fetching missing delegation states", "last_fetched_cycle", e.state.lastFetchedCycle, "last_on_chain_completed_cycle", lastOnChainCompletedCycle)
				}

				for cycle := lastFetchedCycle + 1; cycle <= lastOnChainCompletedCycle; cycle++ {
					// this is not ideal but we can not determine last block on testnets easily
					// so we try to use available last block level, if not fall back to lookup in cycle table which works on mainnet and networks with known parameters by tzgo
					lastBlock := int64(0)
					if cycle == lastOnChainCompletedCycle {
						lastBlock = lastBlockInTheCycle
					}

					if err = e.FetchCycleDelegationStates(e.ctx, cycle, lastBlock, nil); err != nil {
						e.logger.Error("failed to fetch cycle delegation states", "cycle", cycle, "error", err.Error())
					}
					if err = e.store.PruneDelegationState(cycle); err != nil {
						e.logger.Error("failed to prune cycles out", "error", err.Error())
					}
				}

				e.state.SetLastFetchedCycle(lastOnChainCompletedCycle)
			}
		}
	}()
}
