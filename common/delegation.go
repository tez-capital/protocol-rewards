package common

import (
	"github.com/samber/lo"
	"github.com/tez-capital/ogun/constants"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type BalanceUpdateKind string

const (
	KindBalanceUpdateFrozenDeposits   BalanceUpdateKind = "frozen_deposits"
	KindBalanceUpdateUnfrozenDeposits BalanceUpdateKind = "unfrozen_deposits"

	OVERSTAKE_PRECISION = 1_000_000
)

/*
{
  "limit_of_staking_over_baking_millionth": 0,
  "edge_of_baking_over_staking_billionth": 1000000000
}
*/

type StakingParameters struct {
	LimitOfStakingOverBakingMillionth int64 `json:"limit_of_staking_over_baking_millionth"`
	EdgeOfBakingOverStakingBillionth  int64 `json:"edge_of_baking_over_staking_billionth"`
}

type FinalizableUnstakeRequest struct {
	Delegate tezos.Address `json:"delegate"`
	Amount   tezos.Z       `json:"amount"`
	Cycle    int64         `json:"cycle"`
}

type UnfinalizableUnstakeRequests struct {
	Delegate tezos.Address `json:"delegate"`
	Requests []struct {
		Amount tezos.Z `json:"amount"`
		Cycle  int64   `json:"cycle"`
	} `json:"requests"`
}

type UnstakeRequests struct {
	Finalizable   []FinalizableUnstakeRequest  `json:"finalizable"`
	Unfinalizable UnfinalizableUnstakeRequests `json:"unfinalizable"`
}

func (u *UnstakeRequests) GetUnstakedTotalForBaker(baker tezos.Address) int64 {
	total := tezos.Zero
	for _, request := range u.Finalizable {
		if request.Delegate.Equal(baker) {
			total = total.Add(request.Amount)
		}
	}
	if u.Unfinalizable.Delegate.Equal(baker) {
		for _, request := range u.Unfinalizable.Requests {
			total = total.Add(request.Amount)
		}
	}
	return total.Int64()
}

type DelegatorBalances struct {
	DelegatedBalance int64 `json:"delegated_balance"`
	// protion of staked balance included in delegated balance
	OverstakedBalance int64 `json:"overstaked_balance"`
	StakedBalance     int64 `json:"staked_balance"`
}

type DelegatedBalances map[tezos.Address]DelegatorBalances

type DelegationStateBalanceInfo struct {
	Balance         int64         `json:"balance"`
	StakedBalance   int64         `json:"frozen_deposits"`
	UnstakedBalance int64         `json:"unfrozen_deposits"`
	Baker           tezos.Address `json:"delegate"`
	// baker we stake with, can differ in case of delegation change
	StakeBaker tezos.Address `json:"stake_baker"`
}

type DelegationStateBalances map[tezos.Address]DelegationStateBalanceInfo

// TODO: find better name for this
type CreationInfoKind string

const (
	CreatedAtBlockBeginning            CreationInfoKind = "block-beginning"
	CreatedAtBlockMetadata             CreationInfoKind = "block-metadata"
	CreatedAtTransactionMetadata       CreationInfoKind = "transaction-metadata"
	CreatedAtTransactionResult         CreationInfoKind = "transaction-result"
	CreatedAtTransactionInternalResult CreationInfoKind = "transaction-internal-result"

	// special case for delegation
	CreatedOnDelegation CreationInfoKind = "delegation"
)

type DelegationStateCreationInfo struct {
	Level         int64            `json:"level"`
	Operation     tezos.OpHash     `json:"operation"`
	Index         int              `json:"transaction_index"`
	InternalIndex int              `json:"internal_result_index"`
	Kind          CreationInfoKind `json:"kind"`
}

type DelegationState struct {
	Baker      tezos.Address      `json:"baker"`
	Cycle      int64              `json:"cycle"`
	Parameters *StakingParameters `json:"staking_parameters"`

	CreatedAt DelegationStateCreationInfo `json:"created_at"`

	balances DelegationStateBalances
}

func NewDelegationState(delegate *rpc.Delegate, cycle int64) *DelegationState {
	return &DelegationState{
		Baker:    delegate.Delegate,
		Cycle:    cycle,
		balances: make(DelegationStateBalances),
	}
}

func (d *DelegationState) overstakeFactor() tezos.Z {
	bakerStakingBalance := d.GetBakerStakedBalance()
	limit := tezos.NewZ(d.Parameters.LimitOfStakingOverBakingMillionth).Mul64(bakerStakingBalance).Div64(1_000_000)
	stakedBalance := tezos.NewZ(d.GetStakersStakedBalance())
	if stakedBalance.IsLess(limit) {
		return tezos.Zero
	}
	overLimit := stakedBalance.Sub(limit)

	return overLimit.Mul64(OVERSTAKE_PRECISION).Div(stakedBalance)
}

func (d *DelegationState) AddBalance(address tezos.Address, balanceInfo DelegationStateBalanceInfo) {
	d.balances[address] = balanceInfo
}

func (d *DelegationState) UpdateBalance(address tezos.Address, kind string, change int64) error {
	balanceInfo, ok := d.balances[address]
	if !ok {
		return constants.ErrBalanceNotFoundInDelegationState
	}
	switch kind {
	case "unfrozen_deposits":
		balanceInfo.UnstakedBalance += change
	case "frozen_deposits":
		balanceInfo.StakedBalance += change
	default:
		balanceInfo.Balance += change
	}

	d.balances[address] = balanceInfo
	return nil
}

func (d *DelegationState) Delegate(delegator tezos.Address, delegate tezos.Address) error {
	balanceInfo, ok := d.balances[delegator]
	if !ok {
		return constants.ErrDelegatorNotFoundInDelegationState
	}

	balanceInfo.Baker = delegate
	balanceInfo.StakeBaker = delegate
	d.balances[delegator] = balanceInfo
	return nil
}

func (d *DelegationState) GetDelegatorBalanceInfos() DelegationStateBalances {
	result := make(DelegationStateBalances, len(d.balances))
	for addr, balanceInfo := range d.balances {
		if addr.Equal(d.Baker) { // skip baker
			continue
		}
		if !balanceInfo.Baker.Equal(d.Baker) { // skip delegators which left
			continue
		}
		result[addr] = balanceInfo
	}
	return result
}

func (d *DelegationState) GetDelegatedBalance() int64 {
	return lo.Reduce(lo.Values(d.GetDelegatorAndBakerBalances()), func(acc int64, balance DelegatorBalances, _ int) int64 {
		return acc + balance.DelegatedBalance
	}, 0)
}

// includes baker own balance contributing to the total delegated balance
func (d *DelegationState) GetDelegatorAndBakerBalances() DelegatedBalances {
	overstakeFactor := d.overstakeFactor()

	delegators := make(DelegatedBalances, len(d.balances))
	for addr, balanceInfo := range d.balances {
		delegatorBalances := DelegatorBalances{}
		var overstakedBalance int64

		if balanceInfo.Baker.Equal(d.Baker) {
			/* unstaked balance comes from block with minimum which corresponds with d.Baker, not from the actual stake so we include it here */
			delegatorBalances.DelegatedBalance = balanceInfo.Balance + balanceInfo.UnstakedBalance
		}

		if balanceInfo.StakeBaker.Equal(d.Baker) {
			delegatorBalances.StakedBalance = balanceInfo.StakedBalance
			if addr.Equal(d.Baker) { // baker balance is never overstaked
				overstakedBalance = 0
			} else {
				overstakedBalance = overstakeFactor.Mul64(balanceInfo.StakedBalance).Div64(OVERSTAKE_PRECISION).Int64()
			}

			delegatorBalances.OverstakedBalance = overstakedBalance
			delegatorBalances.DelegatedBalance += overstakedBalance // include overstaked balance in delegated balance
		}

		if delegatorBalances.DelegatedBalance+delegatorBalances.StakedBalance == 0 {
			continue // skip empty balances
		}

		delegators[addr] = delegatorBalances
	}
	return delegators
}

func (d *DelegationState) HasContractBalanceInfo(address tezos.Address) bool {
	_, ok := d.balances[address]
	return ok
}

func (d *DelegationState) GetBakerStakedBalance() int64 {
	balanceInfo := d.balances[d.Baker]
	return balanceInfo.StakedBalance
}

func (d *DelegationState) GetStakersStakedBalance() int64 {
	var stakedBalance int64
	for addr, balanceInfo := range d.balances {
		if addr.Equal(d.Baker) {
			continue
		}
		if !balanceInfo.StakeBaker.Equal(d.Baker) {
			continue
		}
		stakedBalance += balanceInfo.StakedBalance
	}
	return stakedBalance
}

func (d *DelegationState) GetBakingPower() int64 {
	balances := d.GetDelegatorAndBakerBalances()
	stakedPower := lo.Reduce(lo.Values(balances), func(acc int64, balance DelegatorBalances, _ int) int64 {
		return acc + balance.StakedBalance
	}, 0)
	delegatedPower := lo.Reduce(lo.Values(balances), func(acc int64, balance DelegatorBalances, _ int) int64 {
		return acc + balance.DelegatedBalance
	}, 0)

	if d.Cycle < 748 {
		return stakedPower + delegatedPower
	}
	return stakedPower + delegatedPower/2
}
