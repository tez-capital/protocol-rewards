package core

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/samber/lo"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type DefaultRpcAndTzktCollector struct {
	rpcUrl string
	rpc    *rpc.Client
	//tzkt *tzkt.Client
}

var (
	defaultCtx context.Context = context.Background()
)

func InitDefaultRpcAndTzktCollector(rpcUrl string) (*DefaultRpcAndTzktCollector, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	rpcClient, err := rpc.NewClient(rpcUrl, &client)
	if err != nil {
		return nil, err
	}

	result := &DefaultRpcAndTzktCollector{
		rpcUrl: rpcUrl,
		rpc:    rpcClient,
	}

	return result, result.RefreshParams()
}

func (engine *DefaultRpcAndTzktCollector) GetId() string {
	return "DefaultRpcAndTzktColletor"
}

func (engine *DefaultRpcAndTzktCollector) RefreshParams() error {
	return engine.rpc.Init(context.Background())
}

func (engine *DefaultRpcAndTzktCollector) GetCurrentProtocol() (tezos.ProtocolHash, error) {
	params, err := engine.rpc.GetParams(context.Background(), rpc.Head)

	if err != nil {
		return tezos.ZeroProtocolHash, err
	}
	return params.Protocol, nil
}

func (engine *DefaultRpcAndTzktCollector) GetCurrentCycleNumber() (int64, error) {
	head, err := engine.rpc.GetHeadBlock(defaultCtx)
	if err != nil {
		return 0, err
	}

	return head.GetLevelInfo().Cycle, err
}

func (engine *DefaultRpcAndTzktCollector) GetLastCompletedCycle() (int64, error) {
	cycle, err := engine.GetCurrentCycleNumber()
	return cycle - 1, err
}

func (engine *DefaultRpcAndTzktCollector) determineLastBlockOfCycle(ctx context.Context, cycle int64) (rpc.BlockID, error) {
	// TODO:
	return rpc.BlockLevel(5777367), nil
}

func (engine *DefaultRpcAndTzktCollector) GetDelegateStateFromCycle(ctx context.Context, cycle int64, delegateAddress tezos.Address) (*rpc.Delegate, error) {
	blockId, err := engine.determineLastBlockOfCycle(ctx, cycle)
	if err != nil {
		return nil, err
	}

	return engine.rpc.GetDelegate(ctx, delegateAddress, blockId)
}

func (engine *DefaultRpcAndTzktCollector) fetchDelegationState(ctx context.Context, delegate *rpc.Delegate, blockId rpc.BlockID) (*DelegationState, error) {
	state := &DelegationState{
		Baker:        delegate.Delegate,
		Balances:     make(map[tezos.Address]tezos.Z, len(delegate.DelegatedContracts)+1),
		TotalBalance: tezos.Z{},
	}

	state.Balances[delegate.Delegate] = tezos.NewZ(delegate.FullBalance - delegate.CurrentFrozenDeposits)

	for _, address := range delegate.DelegatedContracts {
		balance, err := engine.rpc.GetContractBalance(ctx, address, blockId)
		if err != nil {
			return nil, err
		}
		state.Balances[address] = balance
	}

	state.TotalBalance = lo.Reduce(lo.Values(state.Balances), func(acc tezos.Z, balance tezos.Z, _ int) tezos.Z {
		return acc.Add(balance)
	}, state.TotalBalance)

	return state, nil
}

func (engine *DefaultRpcAndTzktCollector) GetDelegationState(ctx context.Context, delegate *rpc.Delegate) (*DelegationState, error) {
	if delegate.MinDelegated.Level.Level == 0 {
		return nil, errors.New("delegate has no minimum delegated balance")
	}

	blockWithMinimumBalance, err := engine.rpc.GetBlock(ctx, rpc.BlockLevel(delegate.MinDelegated.Level.Level))
	if err != nil {
		return nil, err
	}

	state, err := engine.fetchDelegationState(ctx, delegate, rpc.BlockLevel(delegate.MinDelegated.Level.Level-1))
	if err != nil {
		return nil, err
	}

	state.Cycle = delegate.MinDelegated.Level.Cycle
	state.Level = delegate.MinDelegated.Level.Level

	found := false

	allBalanceUpdates := make(ExtendedBalanceUpdates, 0, len(blockWithMinimumBalance.Operations)*2 /* thats minimum of balance updates we expect*/)
	// block balance updates
	allBalanceUpdates = allBalanceUpdates.AddBalanceUpdates(tezos.ZeroOpHash, -1, BlockBalanceUpdateSource, blockWithMinimumBalance.Metadata.BalanceUpdates...)

	for _, batch := range blockWithMinimumBalance.Operations {
		for _, operation := range batch {
			// first op fees
			for transactionIndex, content := range operation.Contents {
				allBalanceUpdates = allBalanceUpdates.AddBalanceUpdates(operation.Hash,
					int64(transactionIndex),
					TransactionMetadataBalanceUpdateSource,
					content.Meta().BalanceUpdates...,
				)
			}
			// then transfers
			for transactionIndex, content := range operation.Contents {
				allBalanceUpdates = allBalanceUpdates.AddBalanceUpdates(operation.Hash,
					int64(transactionIndex),
					TransactionContentsBalanceUpdateSource,
					content.Result().BalanceUpdates...,
				)

				for internalResultIndex, internalResult := range content.Meta().InternalResults {
					slices.Reverse(internalResult.Result.BalanceUpdates)
					allBalanceUpdates = allBalanceUpdates.AddInternalResultBalanceUpdates(operation.Hash,
						state.Index,
						int64(internalResultIndex),
						internalResult.Result.BalanceUpdates...,
					)
				}
			}

		}
	}

	targetAmount := delegate.MinDelegated.Amount

	for _, balanceUpdate := range allBalanceUpdates {
		if _, found := state.Balances[balanceUpdate.Address()]; !found {
			continue
		}

		state.Balances[balanceUpdate.Address()] = state.Balances[balanceUpdate.Address()].Add64(balanceUpdate.Amount())
		state.TotalBalance = state.TotalBalance.Add64(balanceUpdate.Amount())

		if state.TotalBalance.Int64() == targetAmount {
			found = true
			state.Operation = balanceUpdate.Operation
			state.Index = balanceUpdate.Index
			state.InternalIndex = balanceUpdate.InternalIndex
			state.Source = balanceUpdate.Source
			break
		}
	}

	if !found {
		return nil, errors.New("delegate has not reached minimum delegated balance")
	}
	return state, nil
}
