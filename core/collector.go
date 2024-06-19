package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/tez-capital/ogun/common"
	"github.com/tez-capital/ogun/constants"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type rpcCollector struct {
	rpcs []*rpc.Client
}

func attemptWithClients[T interface{}](clients []*rpc.Client, f func(client *rpc.Client) (T, error)) (T, error) {
	var err error
	var result T

	// try 3 times
	for i := 0; i < 3; i++ {
		for _, client := range clients {
			result, err = f(client)
			if err != nil {
				continue
			}
			return result, nil
		}
		// sleep for some time
		sleepTime := (rand.Intn(5)*(i+1) + 5)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
	return result, err
}

func initRpcClient(ctx context.Context, rpcUrl string, transport http.RoundTripper) (*rpc.Client, error) {
	client := http.Client{
		Timeout: constants.HTTP_CLIENT_TIMEOUT_SECONDS * time.Second,
	}

	if transport != nil {
		client.Transport = transport
	}

	rpcClient, err := rpc.NewClient(rpcUrl, &client)
	if err != nil {
		slog.Debug("failed to create rpc client", "url", rpcUrl, "error", err.Error())
		return nil, err
	}
	for i := 0; i < 3; i++ {
		err = rpcClient.Init(ctx)
		if err == nil {
			break
		}
		slog.Debug("failed to init rpc client, retrying", "url", rpcUrl, "error", err.Error())
		time.Sleep(time.Duration(rand.Intn(5)+5) * time.Second)
	}
	if err != nil {
		slog.Debug("failed to init rpc client", "url", rpcUrl, "error", err.Error())
		return nil, err
	}
	return rpcClient, nil
}

func newRpcCollector(ctx context.Context, rpcUrls []string, transport http.RoundTripper) (*rpcCollector, error) {
	result := &rpcCollector{
		rpcs: make([]*rpc.Client, 0, len(rpcUrls)),
	}

	runInParallel(ctx, rpcUrls, constants.RPC_INIT_BATCH_SIZE, func(ctx context.Context, url string, mtx *sync.RWMutex) (cancel bool) {
		client, err := initRpcClient(ctx, url, transport)
		if err != nil {
			return
		}
		mtx.Lock()
		defer mtx.Unlock()
		result.rpcs = append(result.rpcs, client)
		return
	})

	if len(result.rpcs) == 0 {
		return nil, errors.New("no rpc clients available")
	}

	return result, nil
}

func (engine *rpcCollector) getContractFullBalance(ctx context.Context, addr tezos.Address, id rpc.BlockID) (tezos.Z, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/contracts/%s/full_balance", id, addr)
	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		var bal tezos.Z
		err := client.Get(ctx, u, &bal)
		return bal, err
	})
}

func (engine *rpcCollector) getContractStakedBalance(ctx context.Context, addr tezos.Address, id rpc.BlockID) (tezos.Z, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/contracts/%s/staked_balance", id, addr)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		var bal tezos.Z
		err := client.Get(ctx, u, &bal)
		return bal, err
	})
}

func (engine *rpcCollector) getContractDelegate(ctx context.Context, addr tezos.Address, id rpc.BlockID) (tezos.Address, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/contracts/%s/delegate", id, addr)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Address, error) {
		var addr tezos.Address
		err := client.Get(ctx, u, &addr)
		return addr, err
	})
}

func (engine *rpcCollector) getDelegateActiveStakingParameters(ctx context.Context, addr tezos.Address, id rpc.BlockID) (*common.StakingParameters, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/delegates/%s/active_staking_parameters", id, addr)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (*common.StakingParameters, error) {
		var params common.StakingParameters
		err := client.Get(ctx, u, &params)
		return &params, err
	})
}

func (engine *rpcCollector) GetCurrentProtocol() (tezos.ProtocolHash, error) {
	params, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (*tezos.Params, error) {
		return client.GetParams(context.Background(), rpc.Head)
	})
	if err != nil {
		return tezos.ZeroProtocolHash, err
	}
	return params.Protocol, nil
}

func (engine *rpcCollector) GetCurrentCycleNumber(ctx context.Context) (int64, error) {
	head, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Block, error) {
		return client.GetHeadBlock(ctx)
	})
	if err != nil {
		return 0, err
	}

	return head.GetLevelInfo().Cycle, err
}

func (engine *rpcCollector) GetLastCompletedCycle(ctx context.Context) (int64, error) {
	cycle, err := engine.GetCurrentCycleNumber(ctx)
	return cycle - 1, err
}

func (engine *rpcCollector) GetCycleBakingPowerOrigin(ctx context.Context, cycle int64) (originCycle int64) {
	consensusDelay, _ := attemptWithClients(engine.rpcs, func(client *rpc.Client) (int64, error) {
		return client.Params.ConsensusRightsDelay, nil
	})

	// yeah that is a bit counter-intuitive, but at the end of cycle c
	// we compute rights for c+1+consensus_rights_delay
	return cycle - 1 - consensusDelay
}

func (engine *rpcCollector) determineLastBlockOfCycle(cycle int64) rpc.BlockID {
	height, _ := attemptWithClients(engine.rpcs, func(client *rpc.Client) (int64, error) {
		return client.Params.CycleEndHeight(cycle), nil
	})

	return rpc.BlockLevel(height)
}

func (engine *rpcCollector) GetActiveDelegatesFromCycle(ctx context.Context, cycle int64) (rpc.DelegateList, error) {
	id := engine.determineLastBlockOfCycle(cycle)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (rpc.DelegateList, error) {
		return client.ListActiveDelegates(ctx, id)
	})
}

func (engine *rpcCollector) GetDelegateFromCycle(ctx context.Context, cycle int64, delegateAddress tezos.Address) (*rpc.Delegate, error) {
	blockId := engine.determineLastBlockOfCycle(cycle)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Delegate, error) {
		return client.GetDelegate(ctx, delegateAddress, blockId)
	})
}

// fetches the balance of the contract at the beginning of the block - basically the balance of the contract at the end of the previous block
func (engine *rpcCollector) fetchContractInitialBalanceInfo(ctx context.Context, address tezos.Address, blockWithMinimumId rpc.BlockID, lastBlockInCycle rpc.BlockID) (*common.DelegationStateBalanceInfo, error) {
	previousBlockId := rpc.NewBlockOffset(blockWithMinimumId, -1)

	balance, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		return client.GetContractBalance(ctx, address, previousBlockId)
	})
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	delegate, err := engine.getContractDelegate(ctx, address, previousBlockId)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	fullBalance, err := engine.getContractFullBalance(ctx, address, previousBlockId)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	stakedBalance, err := engine.getContractStakedBalance(ctx, address, previousBlockId)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	stakedBalanceAtTheEndOfCycle, err := engine.getContractStakedBalance(ctx, address, lastBlockInCycle)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	return &common.DelegationStateBalanceInfo{
		// actual delegated balance is the balance of the contract plus the sum of the actual amounts of the unfrozen deposits
		Balance:         balance.Int64(),
		StakedBalance:   stakedBalanceAtTheEndOfCycle.Int64(),
		UnstakedBalance: fullBalance.Int64() - stakedBalance.Int64() - balance.Int64(),
		Baker:           delegate,
	}, nil
}

// we fetch the previous block to get the state at the beginning of the block we are going to process
func (engine *rpcCollector) fetchInitialDelegationState(ctx context.Context, delegate *rpc.Delegate, cycle int64, blockWithMinimumId rpc.BlockID) (*common.DelegationState, error) {
	lastBlockInCycle := engine.determineLastBlockOfCycle(cycle)
	state := common.NewDelegationState(delegate, cycle) // initialization has to be from delegate passed here

	// fetch staking parameters, staking parameters are taken from one block before the cycle ends
	params, err := engine.getDelegateActiveStakingParameters(ctx, delegate.Delegate, lastBlockInCycle)
	if err != nil {
		return nil, err
	}
	state.Parameters = params

	// but we fill the rest from delegate state at the beginning of the block
	delegate, err = attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Delegate, error) {
		return client.GetDelegate(ctx, delegate.Delegate, rpc.NewBlockOffset(blockWithMinimumId, -1))
	})
	if err != nil {
		return nil, err
	}

	delegateStakedBalance, err := engine.getContractStakedBalance(ctx, delegate.Delegate, lastBlockInCycle)
	if err != nil {
		return nil, err
	}

	state.AddBalance(delegate.Delegate, common.DelegationStateBalanceInfo{
		Balance:         delegate.Balance,
		StakedBalance:   delegateStakedBalance.Int64(),
		UnstakedBalance: delegate.FullBalance - delegate.CurrentFrozenDeposits - delegate.Balance,
		Baker:           delegate.Delegate,
	})

	toCollect := lo.Filter(delegate.DelegatedContracts, func(address tezos.Address, _ int) bool {
		return address != delegate.Delegate // baker is already included in the state
	})

	for i := 0; i < constants.BALANCE_FETCH_RETRY_ATTEMPTS; i += 1 {
		toCollectNow := toCollect
		toCollect = make([]tezos.Address, 0)
		// add the balance of the delegated contracts
		runInParallel(ctx, toCollectNow, constants.CONTRACT_FETCH_BATCH_SIZE, func(ctx context.Context, address tezos.Address, mtx *sync.RWMutex) (cancel bool) {
			balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, address, blockWithMinimumId, lastBlockInCycle)

			if err != nil {
				slog.Debug("failed to fetch contract balance info", "address", address.String(), "error", err)

				mtx.Lock()
				defer mtx.Unlock()
				toCollect = append(toCollect, address)
				return
			}

			mtx.Lock()
			defer mtx.Unlock()
			state.AddBalance(address, *balanceInfo)
			return
		})

		if len(toCollect) == 0 {
			break
		}

		time.Sleep(constants.BALANCE_FETCH_RETRY_DELAY_SECONDS * time.Second)
	}

	if len(toCollect) > 0 {
		return nil, constants.ErrFailedToFetchContractBalances
	}

	return state, nil
}

func (engine *rpcCollector) getBlockBalanceUpdates(ctx context.Context, state *common.DelegationState, blockLevelWithMinimumBalance rpc.BlockLevel) (OgunBalanceUpdates, error) {
	lastBlockInCycle := engine.determineLastBlockOfCycle(state.Cycle)

	blockWithMinimumBalance, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Block, error) {
		return client.GetBlock(ctx, blockLevelWithMinimumBalance)
	})
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
						slog.Debug("delegation op with invalid content", "operation", operation.Hash)
					}

					if !state.HasContractBalanceInfo(content.Source) {
						// fetch
						balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, content.Source, blockLevelWithMinimumBalance, lastBlockInCycle)
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
					if internalResult.Kind == tezos.OpTypeDelegation {
						if !state.HasContractBalanceInfo(internalResult.Source) {
							// fetch
							balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, internalResult.Source, blockLevelWithMinimumBalance, lastBlockInCycle)
							if err != nil {
								return nil, err
							}
							state.AddBalance(internalResult.Source, *balanceInfo)
						}
						delegate := tezos.ZeroAddress
						if internalResult.Delegate != nil {
							delegate = *internalResult.Delegate
						}

						allBalanceUpdates = allBalanceUpdates.Add(OgunBalanceUpdate{
							Address:   internalResult.Source,
							Operation: operation.Hash,
							Index:     transactionIndex,
							Source:    common.CreatedOnDelegation,
							Delegate:  delegate,
						})
						// no other updates nor internal results for delegation
						continue
					}
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

	// for some reason updates caused by unstake deposits -> deposits are not considered ¯\_(ツ)_/¯
	preprocessedBlockBalanceUpdates := make([]OgunBalanceUpdate, 0, len(blockBalanceUpdates))
	cache := make([]OgunBalanceUpdate, 0)
	skip := false
	for i, update := range blockBalanceUpdates {
		if skip {
			cache = append(cache, update)
			skip = false
			continue
		}
		if i+1 < len(blockBalanceUpdates) {
			next := blockBalanceUpdates[i+1]
			if update.Amount < 0 && next.Kind == "freezer" && next.Category == "deposits" {
				cache = append(cache, update)
				skip = true
				continue
			}
		}
		preprocessedBlockBalanceUpdates = append(preprocessedBlockBalanceUpdates, update)
	}
	//  for some reason updates caused by unstake deposits -> deposits are not considered ¯\_(ツ)_/¯

	// block balance updates last
	allBalanceUpdates = allBalanceUpdates.Add(preprocessedBlockBalanceUpdates...).Add(cache...)

	return allBalanceUpdates, nil
}

func (engine *rpcCollector) GetDelegationState(ctx context.Context, delegate *rpc.Delegate, cycle int64) (*common.DelegationState, error) {
	blockLevelWithMinimumBalance := rpc.BlockLevel(delegate.MinDelegated.Level.Level)
	targetAmount := delegate.MinDelegated.Amount

	if blockLevelWithMinimumBalance == 0 {
		lastBlockInCycle := engine.determineLastBlockOfCycle(cycle)
		slog.Debug("fetching delegation state - no minimum, taking last block balances", "blockLevelWithMinimumBalance", lastBlockInCycle, "delegate", delegate.Delegate.String())
		state, err := engine.fetchInitialDelegationState(ctx, delegate, cycle, lastBlockInCycle)
		if err != nil {
			return nil, err
		}
		return state, constants.ErrDelegateHasNoMinimumDelegatedBalance
	}

	slog.Debug("fetching delegation state", "blockLevelWithMinimumBalance", blockLevelWithMinimumBalance, "delegate", delegate.Delegate.String())
	state, err := engine.fetchInitialDelegationState(ctx, delegate, cycle, blockLevelWithMinimumBalance)
	if err != nil {
		return nil, err
	}

	// we may match at the beginning of the block, we do not have to further process
	if abs(state.DelegatedBalance()-targetAmount) <= constants.OGUN_MINIMUM_DIFF_TOLERANCE {
		state.CreatedAt = common.DelegationStateCreationInfo{
			Level: blockLevelWithMinimumBalance.Int64(),
			Kind:  common.CreatedAtBlockBeginning,
		}
		return state, nil
	}

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

		slog.Debug("balance update", "delegate", balanceUpdate.Delegate, "address", balanceUpdate.Address.String(), "delegated_balance", state.DelegatedBalance(), "amount", balanceUpdate.Amount, "target_amount", targetAmount, "diff", state.DelegatedBalance()-targetAmount)

		if abs(state.DelegatedBalance()-targetAmount) <= constants.OGUN_MINIMUM_DIFF_TOLERANCE {
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
		return nil, constants.ErrMinimumDelegatedBalanceNotFound
	}
	return state, nil
}
