package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/tez-capital/protocol-rewards/common"
	"github.com/tez-capital/protocol-rewards/constants"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type rpcCollector struct {
	rpcs     []*rpc.Client
	tzktUrls []string
	client   *http.Client
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

func newRpcCollector(ctx context.Context, rpcUrls []string, tzktUrls []string, transport http.RoundTripper) (*rpcCollector, error) {
	result := &rpcCollector{
		rpcs:     make([]*rpc.Client, 0, len(rpcUrls)),
		tzktUrls: tzktUrls,
		client: &http.Client{
			Timeout: constants.HTTP_CLIENT_TIMEOUT_SECONDS * time.Second,
		},
	}

	runInParallel(ctx, rpcUrls, constants.RPC_INIT_BATCH_SIZE, func(ctx context.Context, url string, mtx *sync.RWMutex) (cancel bool) {
		mtx.Lock()
		defer mtx.Unlock()
		client, err := initRpcClient(ctx, url, transport)
		if err != nil {
			return
		}
		result.rpcs = append(result.rpcs, client)
		return
	})

	if len(result.rpcs) == 0 {
		return nil, errors.New("no rpc clients available")
	}

	result.tzktUrls = tzktUrls
	result.client.Transport = transport
	return result, nil
}

func (engine *rpcCollector) getContractStakedBalance(ctx context.Context, addr tezos.Address, id rpc.BlockID) (tezos.Z, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/contracts/%s/staked_balance", id, addr)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		var bal tezos.Z
		err := client.Get(ctx, u, &bal)
		return bal, err
	})
}

func (engine *rpcCollector) getDelegatedBalanceFromRawContext(ctx context.Context, delegate tezos.Address, id rpc.BlockID) (tezos.Z, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/raw/json/staking_balance/%s", id, delegate)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		var bal struct {
			Delegated tezos.Z `json:"delegated"`
		}
		err := client.Get(ctx, u, &bal)
		return bal.Delegated, err
	})
}

func (engine *rpcCollector) getContractUnstakeRequests(ctx context.Context, addr tezos.Address, id rpc.BlockID) (common.UnstakeRequests, error) {
	// chains/main/blocks/5896790/context/contracts/tz1epK8fDnc8tUeK6dNwTjiHqrGzX586ozyt/unstake_requests
	u := fmt.Sprintf("chains/main/blocks/%s/context/contracts/%s/unstake_requests", id, addr)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (common.UnstakeRequests, error) {
		var requests common.UnstakeRequests
		err := client.Get(ctx, u, &requests)
		return requests, err
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

func (engine *rpcCollector) getDelegateDelegatedContracts(ctx context.Context, addr tezos.Address, id rpc.BlockID) ([]tezos.Address, error) {
	u := fmt.Sprintf("chains/main/blocks/%s/context/delegates/%s/delegated_contracts", id, addr)

	return attemptWithClients(engine.rpcs, func(client *rpc.Client) ([]tezos.Address, error) {
		var delegatedContracts []tezos.Address
		err := client.Get(ctx, u, &delegatedContracts)
		if err != nil {
			if strings.Contains(err.Error(), "delegate.not_registered") {
				return []tezos.Address{}, constants.ErrDelegateNotRegistered
			}
			var rpcErrors []rpc.GenericError
			err2 := client.Get(ctx, u, &rpcErrors)
			if err2 != nil {
				return nil, err
			}
			for _, rpcError := range rpcErrors {
				if strings.Contains(rpcError.ID, "delegate.not_registered") {
					return []tezos.Address{}, constants.ErrDelegateNotRegistered
				}
			}
		}

		return delegatedContracts, err
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

func (engine *rpcCollector) GetLastCompletedCycle(ctx context.Context) (cycle int64, lastBlockLevel int64, err error) {
	head, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Block, error) {
		return client.GetHeadBlock(ctx)
	})
	if err != nil {
		return 0, 0, err
	}

	levelInfo := head.GetLevelInfo()
	previousCycle := levelInfo.Cycle - 1
	lastBlockInPreviousCycle := head.Header.Level - levelInfo.CyclePosition - 1

	return previousCycle, lastBlockInPreviousCycle, err
}

func (engine *rpcCollector) GetCycleBakingPowerOrigin(ctx context.Context, cycle int64) (originCycle int64) {
	consensusDelay, _ := attemptWithClients(engine.rpcs, func(client *rpc.Client) (int64, error) {
		return client.Params.ConsensusRightsDelay, nil
	})

	// yeah that is a bit counter-intuitive, but at the end of cycle c
	// we compute rights for c+1+consensus_rights_delay
	return cycle - 1 - consensusDelay
}

func (engine *rpcCollector) determineLastBlockOfCycle(cycle int64) int64 {
	height, _ := attemptWithClients(engine.rpcs, func(client *rpc.Client) (int64, error) {
		return client.Params.CycleEndHeight(cycle), nil
	})

	return height
}

func (engine *rpcCollector) GetActiveDelegatesFromCycle(ctx context.Context, lastBlockInTheCycle rpc.BlockID) (rpc.DelegateList, error) {
	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (rpc.DelegateList, error) {
		return client.ListActiveDelegates(ctx, lastBlockInTheCycle)
	})
}

func (engine *rpcCollector) GetDelegateFromCycle(ctx context.Context, lastBlockInTheCycle rpc.BlockID, delegateAddress tezos.Address) (*rpc.Delegate, error) {
	return attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Delegate, error) {
		return client.GetDelegate(ctx, delegateAddress, lastBlockInTheCycle)
	})
}

// fetches the balance of the contract at the beginning of the block - basically the balance of the contract at the end of the previous block
func (engine *rpcCollector) fetchContractInitialBalanceInfo(ctx context.Context, address tezos.Address, baker tezos.Address, blockWithMinimumId rpc.BlockID, lastBlockInCycle rpc.BlockID) (*common.DelegationStateBalanceInfo, error) {
	blockBeforeMinimumId := rpc.NewBlockOffset(blockWithMinimumId, -1)

	balance, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		return client.GetContractBalance(ctx, address, blockBeforeMinimumId)
	})
	if err != nil {
		if httpStatus, ok := err.(rpc.HTTPStatus); ok && httpStatus.StatusCode() == http.StatusNotFound {
			return &common.DelegationStateBalanceInfo{}, nil
		}

		return nil, errors.Join(constants.ErrFailedToFetchContractBalance, err)
	}

	delegate, err := engine.getContractDelegate(ctx, address, blockBeforeMinimumId)
	if err != nil {
		if httpStatus, ok := err.(rpc.HTTPStatus); ok && httpStatus.StatusCode() == http.StatusNotFound {
			delegate = tezos.ZeroAddress // no delegate
		} else {
			return nil, errors.Join(constants.ErrFailedToFetchContract, err)
		}
	}

	unstakeRequests, err := engine.getContractUnstakeRequests(ctx, address, blockBeforeMinimumId)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContractUnstakeRequests, err)
	}

	stakedBalance, err := engine.getContractStakedBalance(ctx, address, lastBlockInCycle)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	stakeDelegate, err := engine.getContractDelegate(ctx, address, lastBlockInCycle)
	if err != nil {
		if httpStatus, ok := err.(rpc.HTTPStatus); ok && httpStatus.StatusCode() == http.StatusNotFound {
			stakeDelegate = tezos.ZeroAddress // no delegate
		} else {
			return nil, errors.Join(constants.ErrFailedToFetchContract, err)
		}
	}

	return &common.DelegationStateBalanceInfo{
		Balance:         balance.Int64(),
		StakedBalance:   stakedBalance.Int64(),
		UnstakedBalance: unstakeRequests.GetUnstakedTotalForBaker(baker),
		Baker:           delegate,
		StakeBaker:      stakeDelegate,
	}, nil
}

func (engine *rpcCollector) getUnstakeRequestsCandidates(delegate tezos.Address, blockLevel int64) ([]tezos.Address, error) {
	var result []tezos.Address
	var err error

	// try 3 times
	for i := 0; i < 3; i++ {
		for _, clientUrl := range engine.tzktUrls {
			url := fmt.Sprintf("%sv1/staking/unstake_requests?firstLevel.le=%d&baker=%s&select=staker.address&staker.ne=%s&staker.null=false&limit=10000", clientUrl, blockLevel, delegate.String(), delegate.String())
			slog.Debug("fetching unstake requests candidates", "url", url)
			response, err := engine.client.Get(url)
			if err != nil {
				continue
			}

			if response.StatusCode/100 != 2 {
				continue
			}
			candidates := make([]string, 0, len(result))
			defer response.Body.Close()
			err = json.NewDecoder(response.Body).Decode(&candidates)

			if err != nil {
				continue
			}
			for _, address := range candidates {
				addr, err := tezos.ParseAddress(address)
				if err != nil {
					continue
				}
				result = append(result, addr)
			}
			return result, nil
		}
		// sleep for some time
		sleepTime := (rand.Intn(5)*(i+1) + 5)
		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
	return result, err
}

// we fetch the previous block to get the state at the beginning of the block we are going to process
func (engine *rpcCollector) fetchInitialDelegationState(ctx context.Context, delegate *rpc.Delegate, cycle int64, lastBlockInTheCycle rpc.BlockID, blockWithMinimumId rpc.BlockID) (*common.DelegationState, error) {
	blockBeforeMinimumId := rpc.NewBlockOffset(blockWithMinimumId, -1)
	state := common.NewDelegationState(delegate, cycle, lastBlockInTheCycle) // initialization has to be from delegate passed here

	// fetch staking parameters, staking parameters are taken from one block before the cycle ends
	params, err := engine.getDelegateActiveStakingParameters(ctx, delegate.Delegate, lastBlockInTheCycle)
	if err != nil {
		return nil, err
	}
	state.Parameters = params

	// but we fill the rest from delegate state at the beginning of the block
	delegateDelegatedContracts, err := engine.getDelegateDelegatedContracts(ctx, delegate.Delegate, blockBeforeMinimumId)
	switch err {
	case constants.ErrDelegateNotRegistered: // ignore
	case nil: // ignore
	default:
		return nil, err
	}
	// get potential unstake requests candidates
	unstakeRequestsCandidates, err := engine.getUnstakeRequestsCandidates(delegate.Delegate, blockWithMinimumId.Int64())
	if err != nil {
		return nil, err
	}
	delegateDelegatedContracts = append(delegateDelegatedContracts, unstakeRequestsCandidates...)

	// there may be new stakers at the end of the cycle so we have to check the end of the cycle as well
	delegateDelegatedContractsAtTheEndOfCycle, err := engine.getDelegateDelegatedContracts(ctx, delegate.Delegate, lastBlockInTheCycle)
	if err != nil {
		return nil, err
	}
	delegateDelegatedContracts = lo.Uniq(append(delegateDelegatedContracts, delegateDelegatedContractsAtTheEndOfCycle...))

	balance, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (tezos.Z, error) {
		return client.GetContractBalance(ctx, delegate.Delegate, blockBeforeMinimumId)
	})
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	// staked balance is taken from the last block of the cycle
	stakedBalance, err := engine.getContractStakedBalance(ctx, delegate.Delegate, lastBlockInTheCycle)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	unstakeRequests, err := engine.getContractUnstakeRequests(ctx, delegate.Delegate, blockBeforeMinimumId)
	if err != nil {
		return nil, errors.Join(constants.ErrFailedToFetchContract, err)
	}

	state.AddBalance(delegate.Delegate, common.DelegationStateBalanceInfo{
		Balance:         balance.Int64(),
		StakedBalance:   stakedBalance.Int64(),
		UnstakedBalance: unstakeRequests.GetUnstakedTotal(),
		Baker:           delegate.Delegate,
		StakeBaker:      delegate.Delegate,
	})

	toCollect := lo.Filter(delegateDelegatedContracts, func(address tezos.Address, _ int) bool {
		return address != delegate.Delegate // baker is already included in the state
	})

	for i := 0; i < constants.BALANCE_FETCH_RETRY_ATTEMPTS; i += 1 {
		toCollectNow := toCollect
		toCollect = make([]tezos.Address, 0)
		// add the balance of the delegated contracts
		runInParallel(ctx, toCollectNow, constants.CONTRACT_FETCH_BATCH_SIZE, func(ctx context.Context, address tezos.Address, mtx *sync.RWMutex) (cancel bool) {
			balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, address, delegate.Delegate, blockWithMinimumId, lastBlockInTheCycle)

			if err != nil {
				slog.Warn("failed to fetch contract balance info", "address", address.String(), "error", err)

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

func makeBurnBalanceUpdatesLast(updates []PRBalanceUpdate) []PRBalanceUpdate {
	notBurns := make([]PRBalanceUpdate, 0, len(updates))
	burns := make([]PRBalanceUpdate, 0, len(updates))

	for i := 1; i < len(updates); i += 2 {
		current := updates[i]
		previous := updates[i-1]
		if current.Kind == "burned" && current.Category == "storage fees" {
			burns = append(burns, previous, current)
		} else {
			notBurns = append(notBurns, previous, current)
		}
	}

	if len(notBurns)+len(burns) != len(updates) {
		panic("invalid balance updates")
	}

	return append(notBurns, burns...)
}

func (engine *rpcCollector) getBlockBalanceUpdates(ctx context.Context, state *common.DelegationState, blockLevelWithMinimumBalance rpc.BlockLevel) (PRBalanceUpdates, error) {
	lastBlockInCycle := state.LastBlockLevel

	blockWithMinimumBalance, err := attemptWithClients(engine.rpcs, func(client *rpc.Client) (*rpc.Block, error) {
		return client.GetBlock(ctx, blockLevelWithMinimumBalance)
	})
	if err != nil {
		return nil, err
	}

	allBalanceUpdates := make(PRBalanceUpdates, 0, len(blockWithMinimumBalance.Operations)*2 /* thats minimum of balance updates we expect*/)
	for _, batch := range blockWithMinimumBalance.Operations {
		for _, operation := range batch {
			// first op fees
			for transactionIndex, content := range operation.Contents {
				allBalanceUpdates = allBalanceUpdates.Add(lo.Map(content.Meta().BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) PRBalanceUpdate {
					return PRBalanceUpdate{
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
						balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, content.Source, state.Baker, blockLevelWithMinimumBalance, lastBlockInCycle)
						if err != nil {
							return nil, err
						}
						state.AddBalance(content.Source, *balanceInfo)
					}

					allBalanceUpdates = allBalanceUpdates.Add(PRBalanceUpdate{
						Address:   content.Source,
						Operation: operation.Hash,
						Index:     transactionIndex,
						Source:    common.CreatedOnDelegation,
						Delegate:  content.Delegate,
					})
					// no other updates nor internal results for delegation
					continue
				}

				allBalanceUpdates = allBalanceUpdates.Add(makeBurnBalanceUpdatesLast(lo.Map(content.Result().BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) PRBalanceUpdate {
					return PRBalanceUpdate{
						Address:   bu.Address(),
						Amount:    bu.Amount(),
						Operation: operation.Hash,
						Index:     transactionIndex,
						Source:    common.CreatedAtTransactionResult,
						Kind:      bu.Kind,
						Category:  bu.Category,
					}
				}))...)

				for internalResultIndex, internalResult := range content.Meta().InternalResults {
					if internalResult.Kind == tezos.OpTypeDelegation {
						if !state.HasContractBalanceInfo(internalResult.Source) {
							// fetch
							balanceInfo, err := engine.fetchContractInitialBalanceInfo(ctx, internalResult.Source, state.Baker, blockLevelWithMinimumBalance, lastBlockInCycle)
							if err != nil {
								return nil, err
							}
							state.AddBalance(internalResult.Source, *balanceInfo)
						}
						delegate := tezos.ZeroAddress
						if internalResult.Delegate != nil {
							delegate = *internalResult.Delegate
						}

						allBalanceUpdates = allBalanceUpdates.Add(PRBalanceUpdate{
							Address:   internalResult.Source,
							Operation: operation.Hash,
							Index:     transactionIndex,
							Source:    common.CreatedOnDelegation,
							Delegate:  delegate,
						})
						// no other updates nor internal results for delegation
						continue
					}
					allBalanceUpdates = allBalanceUpdates.Add(makeBurnBalanceUpdatesLast(lo.Map(internalResult.Result.BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) PRBalanceUpdate {
						return PRBalanceUpdate{
							Address:       bu.Address(),
							Amount:        bu.Amount(),
							Operation:     operation.Hash,
							Index:         transactionIndex,
							InternalIndex: internalResultIndex,
							Source:        common.CreatedAtTransactionInternalResult,
							Kind:          bu.Kind,
							Category:      bu.Category,
						}
					}))...)
				}
			}

		}
	}

	blockBalanceUpdates := lo.Map(blockWithMinimumBalance.Metadata.BalanceUpdates, func(bu rpc.BalanceUpdate, _ int) PRBalanceUpdate {
		return PRBalanceUpdate{
			Address:  bu.Address(),
			Amount:   bu.Amount(),
			Source:   common.CreatedAtBlockMetadata,
			Kind:     bu.Kind,
			Category: bu.Category,
		}
	})

	// for some reason updates caused by unstake deposits -> deposits are not considered ¯\_(ツ)_/¯
	preprocessedBlockBalanceUpdates := make([]PRBalanceUpdate, 0, len(blockBalanceUpdates))
	cache := make([]PRBalanceUpdate, 0)
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

func (engine *rpcCollector) GetDelegationState(ctx context.Context, delegate *rpc.Delegate, cycle int64, lastBlockInTheCycle rpc.BlockID) (*common.DelegationState, error) {
	blockLevelWithMinimumBalance := rpc.BlockLevel(delegate.MinDelegated.Level.Level)
	targetAmount := delegate.MinDelegated.Amount

	if blockLevelWithMinimumBalance == 0 {
		slog.Debug("fetching delegation state - no minimum, taking last block balances", "blockLevelWithMinimumBalance", lastBlockInTheCycle, "delegate", delegate.Delegate.String())
		state, err := engine.fetchInitialDelegationState(ctx, delegate, cycle, lastBlockInTheCycle, lastBlockInTheCycle)
		if err != nil {
			return nil, err
		}
		return state, constants.ErrDelegateHasNoMinimumDelegatedBalance
	}

	slog.Debug("fetching delegation state", "blockLevelWithMinimumBalance", blockLevelWithMinimumBalance, "delegate", delegate.Delegate.String())
	state, err := engine.fetchInitialDelegationState(ctx, delegate, cycle, lastBlockInTheCycle, blockLevelWithMinimumBalance)
	if err != nil {
		return nil, err
	}

	// we may match at the beginning of the block, we do not have to further process
	if abs(state.GetDelegatedBalance()-targetAmount) <= constants.MINIMUM_DIFF_TOLERANCE {
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
			case balanceUpdate.Kind == "staking":
				// ignore -> staking balance is determined based on the last block of the cycle, we do not care about the intermediate values
			case balanceUpdate.Kind == "freezer" && balanceUpdate.Category == "deposits":
				// we ignore deposits because only staked balance at the last block of the cycle is important
				//state.UpdateBalance(balanceUpdate.Address, "frozen_deposits", balanceUpdate.Amount)
			case balanceUpdate.Kind == "freezer" && balanceUpdate.Category == "unstaked_deposits":
				state.UpdateBalance(balanceUpdate.Address, "unfrozen_deposits", balanceUpdate.Amount)
			default:
				state.UpdateBalance(balanceUpdate.Address, "", balanceUpdate.Amount)
			}
		}

		slog.Debug("balance update", "delegate", balanceUpdate.Delegate, "address", balanceUpdate.Address.String(), "delegated_balance", state.GetDelegatedBalance(), "amount", balanceUpdate.Amount, "target_amount", targetAmount, "diff", state.GetDelegatedBalance()-targetAmount)

		if abs(state.GetDelegatedBalance()-targetAmount) <= constants.MINIMUM_DIFF_TOLERANCE {
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
