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
	bakerStakingBalance := d.BakerStakingBalance()
	limit := tezos.NewZ(d.Parameters.LimitOfStakingOverBakingMillionth).Mul64(bakerStakingBalance).Div64(1_000_000)
	stakersStakedBalance := tezos.NewZ(d.StakersStakedBalance())
	if stakersStakedBalance.IsLess(limit) {
		return tezos.Zero
	}
	overLimit := stakersStakedBalance.Sub(limit)

	return overLimit.Mul64(OVERSTAKE_PRECISION).Div(stakersStakedBalance)
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
	d.balances[delegator] = balanceInfo
	return nil
}

func (d *DelegationState) DelegatorBalanceInfos() DelegationStateBalances {
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

func (d *DelegationState) DelegatedBalance() int64 {
	overstakeFactor := d.overstakeFactor()

	bakerDelegatedBalance := d.BakerDelegatedBalance()

	delegatedBalance := lo.Reduce(lo.Values(d.DelegatorBalanceInfos()), func(acc int64, balanceInfo DelegationStateBalanceInfo, _ int) int64 {
		return acc + balanceInfo.Balance + balanceInfo.UnstakedBalance + overstakeFactor.Mul64(balanceInfo.StakedBalance).Div64(OVERSTAKE_PRECISION).Int64()
	}, bakerDelegatedBalance)

	return delegatedBalance
}

// includes baker own balance contributing to the total delegated balance
func (d *DelegationState) DelegatorBalances() DelegatedBalances {
	overstakeFactor := d.overstakeFactor()

	delegators := make(DelegatedBalances, len(d.balances))
	for addr, balanceInfo := range d.balances {
		overstakedBalance := overstakeFactor.Mul64(balanceInfo.StakedBalance).Div64(OVERSTAKE_PRECISION).Int64()

		delegators[addr] = DelegatorBalances{
			DelegatedBalance:  balanceInfo.Balance + balanceInfo.UnstakedBalance + overstakedBalance,
			OverstakedBalance: overstakedBalance,
			StakedBalance:     balanceInfo.StakedBalance,
		}
	}
	return delegators
}

func (d *DelegationState) HasContractBalanceInfo(address tezos.Address) bool {
	_, ok := d.balances[address]
	return ok
}

func (d *DelegationState) BakerDelegatedBalance() int64 {
	balanceInfo := d.balances[d.Baker]
	return balanceInfo.Balance + balanceInfo.UnstakedBalance
}

func (d *DelegationState) BakerStakingBalance() int64 {
	balanceInfo := d.balances[d.Baker]
	return balanceInfo.StakedBalance
}

func (d *DelegationState) StakersStakedBalance() int64 {
	var stakedBalance int64
	for addr, balanceInfo := range d.balances {
		if addr.Equal(d.Baker) {
			continue
		}
		stakedBalance += balanceInfo.StakedBalance
	}
	return stakedBalance
}
