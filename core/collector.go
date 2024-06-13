package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
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
	return rpc.Head, nil
}

func (engine *DefaultRpcAndTzktColletor) GetDelegateStateFromCycle(ctx context.Context, cycle int64, delegateAddress tezos.Address) (*rpc.Delegate, error) {
	blockId, err := engine.determineLastBlockOfCycle(ctx, cycle)
	if err != nil {
		return nil, err
	}

	return engine.rpc.GetDelegate(ctx, delegateAddress, blockId)
}

func (engine *DefaultRpcAndTzktColletor) fetchDelegatorBalances(ctx context.Context, state *DelegationState, blockId rpc.BlockID) (totalBalance tezos.Z, err error) {
	totalBalance = tezos.NewZ(state.Balance)
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

	delegatedContracts := state.DelegatedContracts

	sort.Slice(delegatedContracts, func(i, j int) bool {
		return delegatedContracts[i].String() < delegatedContracts[j].String()
	})

	found := false
	// TODO:
	// not blockWithMinimumBalance.Metadata.BalanceUpdates
	// rather loop through operations in blockWithMinimumBalance.Operations
	// process first blockWithMinimumBalance.Operations[0][0].Contents[0].Result().BalanceUpdates
	// then internal results:
	// blockWithMinimumBalance.Operations[0][0].Contents[0].Meta().InternalResults[0].Result.BalanceUpdates
	// ofc not 0 indexes ;-)

	for _, balanceUpdate := range blockWithMinimumBalance.Metadata.BalanceUpdates {
		fmt.Println(totalBalance.Int64(), state.MinDelegated.Amount, balanceUpdate.Amount())
		if totalBalance.Int64() == state.MinDelegated.Amount {
			found = true
			break
		}
		i := sort.Search(len(delegatedContracts), func(i int) bool {
			return delegatedContracts[i].Equal(balanceUpdate.Address())
		})

		if i < len(delegatedContracts) {
			state.DelegatorBalances[delegatedContracts[i]] = state.DelegatorBalances[delegatedContracts[i]].Add64(balanceUpdate.Amount())
			totalBalance = totalBalance.Add64(balanceUpdate.Amount())
		}
	}

	if !found {
		// TODO: remove panic
		panic("Delegate has not reached minimum delegated balance")
		return nil, errors.New("Delegate has not reached minimum delegated balance")
	}
	return state, nil
}
