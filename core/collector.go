package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/tez-capital/ogun/common"
	"github.com/tez-capital/ogun/constants"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

var (
	defaultCtx context.Context = context.Background()
)

func Abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

type DefaultRpcCollector struct {
	rpcUrl string
	rpc    *rpc.Client
}

func NewDefaultRpcCollector(rpcUrl string, transport http.RoundTripper) (*DefaultRpcCollector, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	if transport != nil {
		client.Transport = transport
	}

	rpcClient, err := rpc.NewClient(rpcUrl, &client)
	if err != nil {
		return nil, err
	}
	rpcClient.Init(defaultCtx)

	result := &DefaultRpcCollector{
		rpcUrl: rpcUrl,
		rpc:    rpcClient,
	}

	return result, result.RefreshParams()
}

func (engine *DefaultRpcCollector) RefreshParams() error {
	return engine.rpc.Init(context.Background())
}

func (engine *DefaultRpcCollector) GetCurrentProtocol() (tezos.ProtocolHash, error) {
	params, err := engine.rpc.GetParams(context.Background(), rpc.Head)

	if err != nil {
		return tezos.ZeroProtocolHash, err
	}
	return params.Protocol, nil
}

func (engine *DefaultRpcCollector) GetCurrentCycleNumber(ctx context.Context) (int64, error) {
	head, err := engine.rpc.GetHeadBlock(ctx)
	if err != nil {
		return 0, err
	}

	return head.GetLevelInfo().Cycle, err
}

func (engine *DefaultRpcCollector) GetLastCompletedCycle(ctx context.Context) (int64, error) {
	cycle, err := engine.GetCurrentCycleNumber(ctx)
	return cycle - 1, err
}

func (engine *DefaultRpcCollector) determineLastBlockOfCycle(cycle int64) rpc.BlockID {
	height := engine.rpc.Params.CycleEndHeight(cycle)
	return rpc.BlockLevel(height)
}

func (engine *DefaultRpcCollector) GetActiveDelegatesFromCycle(ctx context.Context, cycle int64) (rpc.DelegateList, error) {
	id := engine.determineLastBlockOfCycle(cycle)
	selector := "active=true&with_minimal_stake=true"
	delegates := make(rpc.DelegateList, 0)
	u := fmt.Sprintf("chains/main/blocks/%s/context/delegates?%s", id, selector)
	if err := engine.rpc.Get(ctx, u, &delegates); err != nil {
		return nil, err
	}
	return delegates, nil
}

func (engine *DefaultRpcCollector) GetDelegateFromCycle(ctx context.Context, cycle int64, delegateAddress tezos.Address) (*rpc.Delegate, error) {
	blockId := engine.determineLastBlockOfCycle(cycle)

	return engine.rpc.GetDelegate(ctx, delegateAddress, blockId)
}

// fetches the balance of the contract at the beginning of the block - basically the balance of the contract at the end of the previous block
func (engine *DefaultRpcCollector) fetchContractInitialBalanceInfo(ctx context.Context, address tezos.Address, blockWithMinimumId rpc.BlockID) (*common.DelegationStateBalanceInfo, error) {
	previousBlockId := rpc.NewBlockOffset(blockWithMinimumId, -1)

	contractInfo, err := engine.rpc.GetContract(ctx, address, previousBlockId)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	return &common.DelegationStateBalanceInfo{
		// actual delegated balance is the balance of the contract plus the sum of the actual amounts of the unfrozen deposits
		Balance:        contractInfo.Balance,
		FrozenDeposits: contractInfo.FrozenDeposits.ActualAmount,
		UnfrozenDeposits: lo.SumBy(contractInfo.UnstakedFrozenDeposits, func(f rpc.UnstakedDeposit) int64 {
			return f.ActualAmount
		}),
		Baker: contractInfo.Delegate,
	}, nil
}

// we fetch the previous block to get the state at the beginning of the block we are going to process
func (engine *DefaultRpcCollector) fetchInitialDelegationState(ctx context.Context, delegate *rpc.Delegate, blockWithMinimumId rpc.BlockID) (*common.DelegationState, error) {
	delegate, err := engine.rpc.GetDelegate(ctx, delegate.Delegate, rpc.NewBlockOffset(blockWithMinimumId, -1))
	if err != nil {
		return nil, err
	}

	state := common.NewDelegationState(delegate)
	state.AddBalance(delegate.Delegate, common.DelegationStateBalanceInfo{
		Balance:          delegate.Balance,
		FrozenDeposits:   delegate.CurrentFrozenDeposits,
		UnfrozenDeposits: delegate.FullBalance - delegate.CurrentFrozenDeposits - delegate.Balance,
		Baker:            delegate.Delegate,
	})

	ch := lo.SliceToChannel(constants.CONTRACT_FETCH_BATCH_SIZE, lo.Filter(delegate.DelegatedContracts, func(address tezos.Address, _ int) bool {
		return address != delegate.Delegate // baker is already included in the state
	}))

	// add the balance of the delegated contracts

	mtx := sync.Mutex{}
	// add the balance of the delegated contracts
	for {
		delegatedContracts, length, _, ok := lo.Buffer(ch, constants.CONTRACT_FETCH_BATCH_SIZE)
		wg := sync.WaitGroup{}
		wg.Add(length)
		for _, address := range delegatedContracts {
			go func(address tezos.Address) {
				defer wg.Done()
				balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, address, blockWithMinimumId)
				if err != nil {
					slog.Error("failed to fetch contract balance info", "address", address.String(), "error", err)
					return
				}
				//fmt.Println("balanceInfo", balanceInfo, "address", address.String())

				mtx.Lock()
				defer mtx.Unlock()
				state.AddBalance(address, *balanceInfo)
			}(address)
		}
		wg.Wait()
		if !ok {
			break
		}
	}
	return state, nil
}

func (engine *DefaultRpcCollector) getBlockBalanceUpdates(ctx context.Context, state *common.DelegationState, blockLevelWithMinimumBalance rpc.BlockLevel) (OgunBalanceUpdates, error) {
	blockWithMinimumBalance, err := engine.rpc.GetBlock(ctx, blockLevelWithMinimumBalance)
	if err != nil {
		return nil, err
	}

	allBalanceUpdates := make(OgunBalanceUpdates, 0, len(blockWithMinimumBalance.Operations)*2 /* thats minimum of balance updates we expect*/)
	for _, batch := range blockWithMinimumBalance.Operations {
		for _, operation := range batch {
			// first op fees
			for transactionIndex, content := range operation.Contents {
				allBalanceUpdates = allBalanceUpdates.Add(lo.Map(content.Meta().BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) OgunBalanceUpdate {
					return OgunBalanceUpdate{
						Address:   bu.Address(),
						Amount:    bu.Amount(),
						Operation: operation.Hash,
						Index:     transactionIndex,
						Source:    common.CreatedAtTransactionMetadata,
						Kind:      bu.Kind,
						Category:  bu.Category,
					}
				})...)
			}
			// then transfers
			for transactionIndex, content := range operation.Contents {
				if content.Kind() == tezos.OpTypeDelegation {
					content, ok := content.(*rpc.Delegation)
					if !ok {
						slog.Error("delegation op with invalid content", "operation", operation.Hash)
					}

					if !state.HasContractBalanceInfo(content.Source) {
						// fetch
						balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, content.Source, blockLevelWithMinimumBalance)
						if err != nil {
							return nil, err
						}
						state.AddBalance(content.Source, *balanceInfo)
					}

					allBalanceUpdates = allBalanceUpdates.Add(OgunBalanceUpdate{
						Address:   content.Source,
						Operation: operation.Hash,
						Index:     transactionIndex,
						Source:    common.CreatedOnDelegation,
						Delegate:  content.Delegate,
					})
					// no other updates nor internal results for delegation
					continue
				}

				allBalanceUpdates = allBalanceUpdates.Add(lo.Map(content.Result().BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) OgunBalanceUpdate {
					return OgunBalanceUpdate{
						Address:   bu.Address(),
						Amount:    bu.Amount(),
						Operation: operation.Hash,
						Index:     transactionIndex,
						Source:    common.CreatedAtTransactionResult,
						Kind:      bu.Kind,
						Category:  bu.Category,
					}
				})...)

				for internalResultIndex, internalResult := range content.Meta().InternalResults {
					allBalanceUpdates = allBalanceUpdates.Add(lo.Map(internalResult.Result.BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) OgunBalanceUpdate {
						return OgunBalanceUpdate{
							Address:       bu.Address(),
							Amount:        bu.Amount(),
							Operation:     operation.Hash,
							Index:         transactionIndex,
							InternalIndex: internalResultIndex,
							Source:        common.CreatedAtTransactionInternalResult,
							Kind:          bu.Kind,
							Category:      bu.Category,
						}
					})...)
				}
			}

		}
	}

	blockBalanceUpdates := lo.Map(blockWithMinimumBalance.Metadata.BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) OgunBalanceUpdate {
		return OgunBalanceUpdate{
			Address:  bu.Address(),
			Amount:   bu.Amount(),
			Source:   common.CreatedAtBlockMetadata,
			Kind:     bu.Kind,
			Category: bu.Category,
		}
	})

	// for some reason updates causes deposits are not considered ¯\_(ツ)_/¯
	preprocessedBlockBalanceUpdates := make([]OgunBalanceUpdate, 0, len(blockBalanceUpdates))
	skip := false
	for i, update := range blockBalanceUpdates {
		if skip {
			skip = false
			continue
		}
		if i+1 < len(blockBalanceUpdates) {
			next := blockBalanceUpdates[i+1]
			if update.Amount < 0 && next.Kind == "freezer" && next.Category == "deposits" {
				skip = true
				continue
			}
		}
		preprocessedBlockBalanceUpdates = append(preprocessedBlockBalanceUpdates, update)
	}
	// end for some reason updates causes deposits are not considered  ¯\_(ツ)_/¯

	// block balance updates last
	allBalanceUpdates = allBalanceUpdates.Add(preprocessedBlockBalanceUpdates...)

	return allBalanceUpdates, nil
}

func (engine *DefaultRpcCollector) GetDelegationState(ctx context.Context, delegate *rpc.Delegate) (*common.DelegationState, error) {
	blockLevelWithMinimumBalance := rpc.BlockLevel(delegate.MinDelegated.Level.Level)
	targetAmount := delegate.MinDelegated.Amount

	if blockLevelWithMinimumBalance == 0 {
		return nil, constants.ErrDelegateHasNoMinimumDelegatedBalance
	}

	state, err := engine.fetchInitialDelegationState(ctx, delegate, blockLevelWithMinimumBalance)
	if err != nil {
		return nil, err
	}

	// we may match at the beginning of the block, we do not have to further process
	if state.DelegatedBalance() == targetAmount {
		state.CreatedAt = common.DelegationStateCreationInfo{
			Level: blockLevelWithMinimumBalance.Int64(),
			Kind:  common.CreatedAtBlockBeginning,
		}
		return state, nil
	}

	// TODO:
	// adjust based on overstake
	allBalanceUpdates, err := engine.getBlockBalanceUpdates(ctx, state, blockLevelWithMinimumBalance)
	if err != nil {
		return nil, err
	}

	found := false
	for _, balanceUpdate := range allBalanceUpdates {
		if !state.HasContractBalanceInfo(balanceUpdate.Address) {
			continue
		}

		if constants.IgnoredBalanceUpdateKinds.Contains(balanceUpdate.Kind) {
			continue
		}

		switch balanceUpdate.Source {
		case common.CreatedOnDelegation:
			state.Delegate(balanceUpdate.Address, balanceUpdate.Delegate)
		default:
			switch {
			case balanceUpdate.Kind == "freezer" && balanceUpdate.Category == "deposits":
				state.UpdateBalance(balanceUpdate.Address, "frozen_deposits", balanceUpdate.Amount)
			case balanceUpdate.Kind == "freezer" && balanceUpdate.Category == "unstaked_deposits":
				state.UpdateBalance(balanceUpdate.Address, "unfrozen_deposits", balanceUpdate.Amount)
			default:
				state.UpdateBalance(balanceUpdate.Address, "", balanceUpdate.Amount)
			}

		}

		//fmt.Println(balanceUpdate.Amount, "====>", state.DelegatedBalance(), targetAmount, state.DelegatedBalance()-targetAmount)

		if Abs(state.DelegatedBalance()-targetAmount) <= 1 {
			found = true
			state.CreatedAt = common.DelegationStateCreationInfo{
				Level:         blockLevelWithMinimumBalance.Int64(),
				Operation:     balanceUpdate.Operation,
				Index:         balanceUpdate.Index,
				InternalIndex: balanceUpdate.InternalIndex,
				Kind:          balanceUpdate.Source,
			}
			break
		}
	}

	if !found {
		slog.Error("failed to find the exact balance", "delegate", delegate.Delegate.String(), "level_info", delegate.MinDelegated.Level, "target", targetAmount, "actual", state.DelegatedBalance())
		return nil, constants.ErrMinimumDelegatedBalanceNotFound
	}
	return state, nil
}
