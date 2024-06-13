package core

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type DefaultRpcAndTzktColletor struct {
	rpcUrl string
	rpc    *rpc.Client
	//tzkt *tzkt.Client
}

var (
	defaultCtx context.Context = context.Background()
)

func InitDefaultRpcAndTzktColletor(rpcUrl string) (*DefaultRpcAndTzktColletor, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	rpcClient, err := rpc.NewClient(rpcUrl, &client)
	if err != nil {
		return nil, err
	}

	result := &DefaultRpcAndTzktColletor{
		rpcUrl: rpcUrl,
		rpc:    rpcClient,
	}

	return result, result.RefreshParams()
}

func (engine *DefaultRpcAndTzktColletor) GetId() string {
	return "DefaultRpcAndTzktColletor"
}

func (engine *DefaultRpcAndTzktColletor) RefreshParams() error {
	return engine.rpc.Init(context.Background())
}

func (engine *DefaultRpcAndTzktColletor) GetCurrentProtocol() (tezos.ProtocolHash, error) {
	params, err := engine.rpc.GetParams(context.Background(), rpc.Head)

	if err != nil {
		return tezos.ZeroProtocolHash, err
	}
	return params.Protocol, nil
}

func (engine *DefaultRpcAndTzktColletor) GetCurrentCycleNumber() (int64, error) {
	head, err := engine.rpc.GetHeadBlock(defaultCtx)
	if err != nil {
		return 0, err
	}

	return head.GetLevelInfo().Cycle, err
}

func (engine *DefaultRpcAndTzktColletor) GetLastCompletedCycle() (int64, error) {
	cycle, err := engine.GetCurrentCycleNumber()
	return cycle - 1, err
}

func (engine *DefaultRpcAndTzktColletor) determineLastBlockOfCycle(ctx context.Context, cycle int64) (rpc.BlockID, error) {
	// TODO:
	return rpc.BlockLevel(5777367), nil
}

func (engine *DefaultRpcAndTzktColletor) GetDelegateStateFromCycle(ctx context.Context, cycle int64, delegateAddress tezos.Address) (*rpc.Delegate, error) {
	blockId, err := engine.determineLastBlockOfCycle(ctx, cycle)
	if err != nil {
		return nil, err
	}

	return engine.rpc.GetDelegate(ctx, delegateAddress, blockId)
}

func (engine *DefaultRpcAndTzktColletor) fetchDelegatorBalances(ctx context.Context, state *DelegationState, blockId rpc.BlockID) (totalBalance tezos.Z, err error) {
	balance, err := engine.rpc.GetContractBalance(ctx, state.Delegate.Delegate, blockId)
	if err != nil {
		return tezos.Zero, err
	}
	state.Balance = balance.Int64()
	totalBalance = tezos.NewZ(state.FullBalance - state.CurrentFrozenDeposits)

	//	totalBalance.Add64(state.CurrentFrozenDeposits)

	for _, address := range state.DelegatedContracts {
		balance, err := engine.rpc.GetContractBalance(ctx, address, blockId)
		if err != nil {
			return tezos.Zero, err
		}
		state.DelegatorBalances[address] = balance
		totalBalance = totalBalance.Add(balance)
	}

	return totalBalance, nil
}

func (engine *DefaultRpcAndTzktColletor) GetDelegationState(ctx context.Context, delegate *rpc.Delegate) (*DelegationState, error) {
	fmt.Println(delegate.MinDelegated)
	if delegate.MinDelegated.Level.Level == 0 {
		return nil, errors.New("Delegate has no minimum delegated balance")
	}

	blockWithMinimumBalance, err := engine.rpc.GetBlock(ctx, rpc.BlockLevel(delegate.MinDelegated.Level.Level))
	if err != nil {
		return nil, err
	}

	state := &DelegationState{
		Delegate: delegate,

		DelegatorBalances: make(map[tezos.Address]tezos.Z, len(delegate.DelegatedContracts)),
	}

	totalBalance, err := engine.fetchDelegatorBalances(ctx, state, rpc.BlockLevel(delegate.MinDelegated.Level.Level-1))
	if err != nil {
		return nil, err
	}

	fmt.Println("total:", totalBalance)
	//totalBalance = tezos.NewZ(297257208061)
	balances := maps.Clone(state.DelegatorBalances)
	balances[delegate.Delegate] = tezos.NewZ(state.Balance)

	found := false

	allBalanceUpdates := make([]rpc.BalanceUpdate, 0, len(blockWithMinimumBalance.Operations)*2 /* thats minimum of balance updates we expect*/)
	// block balance updates
	allBalanceUpdates = append(allBalanceUpdates, blockWithMinimumBalance.Metadata.BalanceUpdates...)

	for _, batch := range blockWithMinimumBalance.Operations {
		for _, operation := range batch {
			// first op fees
			for _, content := range operation.Contents {
				allBalanceUpdates = append(allBalanceUpdates, content.Meta().BalanceUpdates...)
			}
			// then transfers
			for _, content := range operation.Contents {
				allBalanceUpdates = append(allBalanceUpdates, content.Result().BalanceUpdates...)

				for _, internalResult := range content.Meta().InternalResults {
					slices.Reverse(internalResult.Result.BalanceUpdates)
					allBalanceUpdates = append(allBalanceUpdates, internalResult.Result.BalanceUpdates...)
				}
			}

		}
	}

	for _, balanceUpdate := range allBalanceUpdates {
		if totalBalance.Int64() == state.MinDelegated.Amount {
			found = true
			break
		}

		if _, found := balances[balanceUpdate.Address()]; !found {
			continue
		}

		fmt.Println(balanceUpdate.Address())
		fmt.Println("total:", totalBalance, "expected:", state.MinDelegated.Amount, totalBalance.Int64()-state.MinDelegated.Amount)
		fmt.Println("change:", balanceUpdate.Amount())
		balances[balanceUpdate.Address()] = balances[balanceUpdate.Address()].Add64(balanceUpdate.Amount())
		totalBalance = totalBalance.Add64(balanceUpdate.Amount())
	}

	if !found {
		// TODO: remove panic
		panic("Delegate has not reached minimum delegated balance")
		return nil, errors.New("Delegate has not reached minimum delegated balance")
	}
	return state, nil
}
