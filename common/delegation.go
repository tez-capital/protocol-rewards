package common

import (
	"github.com/tez-capital/ogun/constants"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

type DelegatedBalances map[tezos.Address]int64

type DelegationStateBalanceInfo struct {
	Balance          int64         `json:"balance"`
	FrozenDeposits   int64         `json:"frozen_deposits"`
	UnfrozenDeposits int64         `json:"unfrozen_deposits"`
	Baker            tezos.Address `json:"delegate"`
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
	Baker tezos.Address `json:"baker"`
	Cycle int64         `json:"cycle"`

	CreatedAt DelegationStateCreationInfo `json:"created_at"`

	balances         DelegationStateBalances
	delegatedBalance int64
}

func NewDelegationState(delegate *rpc.Delegate) *DelegationState {
	return &DelegationState{
		Baker:    delegate.Delegate,
		Cycle:    delegate.MinDelegated.Level.Cycle,
		balances: make(DelegationStateBalances),
	}
}

func (d *DelegationState) AddBalance(address tezos.Address, balanceInfo DelegationStateBalanceInfo) {
	d.balances[address] = balanceInfo

	if balanceInfo.Baker.Equal(d.Baker) {
		d.delegatedBalance += balanceInfo.Balance + balanceInfo.UnfrozenDeposits
	}
}

func (d *DelegationState) UpdateBalance(address tezos.Address, kind string, change int64) error {
	balanceInfo, ok := d.balances[address]
	if !ok {
		return constants.ErrBalanceNotFoundInDelegationState
	}
	switch kind {
	case "unfrozen_deposits":
		balanceInfo.UnfrozenDeposits += change
	case "frozen_deposits":
		balanceInfo.FrozenDeposits += change
	default:
		balanceInfo.Balance += change
	}

	d.balances[address] = balanceInfo

	if balanceInfo.Baker.Equal(d.Baker) {
		switch kind {
		case "frozen_deposits": // frozen deposits are not included in the delegated balance
		case "unfrozen_deposits":
			fallthrough
		default:
			d.delegatedBalance += change
		}
	}

	return nil
}

func (d *DelegationState) Delegate(delegator tezos.Address, delegate tezos.Address) error {
	balanceInfo, ok := d.balances[delegator]
	if !ok {
		return constants.ErrDelegatorNotFoundInDelegationState
	}

	switch {
	case balanceInfo.Baker.Equal(delegate):
		return nil
	case delegate.Equal(d.Baker):
		d.delegatedBalance += balanceInfo.Balance + balanceInfo.UnfrozenDeposits
	default:
		d.delegatedBalance -= balanceInfo.Balance + balanceInfo.UnfrozenDeposits
	}

	balanceInfo.Baker = delegate
	d.balances[delegator] = balanceInfo
	return nil
}

func (d *DelegationState) DelegatedBalance() int64 {
	return d.delegatedBalance
}

// includes baker own balance contributing to the total delegated balance
func (d *DelegationState) DelegatorDelegatedBalances() DelegatedBalances {
	delegators := make(DelegatedBalances, len(d.balances))
	for _, balanceInfo := range d.balances {
		if !balanceInfo.Baker.Equal(d.Baker) { // skip balances delegated to others
			continue
		}
		delegators[balanceInfo.Baker] = balanceInfo.Balance + balanceInfo.UnfrozenDeposits
	}
	return delegators
}

func (d *DelegationState) HasContractBalanceInfo(address tezos.Address) bool {
	_, ok := d.balances[address]
	return ok
}

func (d *DelegationState) BakerDelegatedBalance() int64 {
	balanceInfo := d.balances[d.Baker]
	return balanceInfo.Balance + balanceInfo.UnfrozenDeposits
}
