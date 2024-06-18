package store

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/tez-capital/ogun/common"
	"github.com/trilitech/tzgo/tezos"
)

type DelegationStateStatus int

const (
	DelegationStateStatusOk                  DelegationStateStatus = iota
	DelegationStateStatusMinimumNotAvailable                       // 1
)

type DelegationStateBalances common.DelegatedBalances

func (j DelegationStateBalances) Value() (driver.Value, error) {
	result, err := json.Marshal(j)
	return string(result), err
}

func (j *DelegationStateBalances) Scan(src interface{}) error {
	if srcTmp, ok := src.(string); ok {
		src = []byte(srcTmp)
	}
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}
	return json.Unmarshal(source, j)
}

type Address struct {
	tezos.Address
}

func (a Address) Value() (driver.Value, error) {
	return a.Address.String(), nil
}

func (a *Address) Scan(src interface{}) error {
	if srcTmp, ok := src.(string); ok {
		src = []byte(srcTmp)
	}
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}
	addr, err := tezos.ParseAddress(string(source))
	if err != nil {
		return err
	}
	a.Address = addr
	return nil
}

type TzktDelegator struct {
	Address          tezos.Address `json:"address"`
	DelegatedBalance int64         `json:"delegatedBalance"`
	StakedBalance    int64         `json:"stakedBalance"`
}

type TzktLikeDelegationState struct {
	Cycle                    int64           `json:"cycle"`
	OwnDelegatedBalance      int64           `json:"ownDelegatedBalance"`
	OwnStakedBalance         int64           `json:"ownStakedBalance"`
	ExternalDelegatedBalance int64           `json:"externalDelegatedBalance"`
	ExternalStakedBalance    int64           `json:"externalStakedBalance"`
	DelegatorsCount          int             `json:"delegatorsCount"`
	Delegators               []TzktDelegator `json:"delegators"`
}

type StoredDelegationState struct {
	Delegate Address                 `json:"delegate" gorm:"primaryKey"`
	Cycle    int64                   `json:"cycle" gorm:"primaryKey"`
	Status   DelegationStateStatus   `json:"status"`
	Balances DelegationStateBalances `json:"balances" gorm:"type:jsonb;default:'{}'"`
}

func (s *StoredDelegationState) OwnDelegatedbalance() common.DelegatorBalances {
	return s.Balances[s.Delegate.Address]
}

func (s *StoredDelegationState) ExternalDelegatedBalance() common.DelegatorBalances {
	result := common.DelegatorBalances{}
	for addr, balances := range s.Balances {
		if addr != s.Delegate.Address {
			result.DelegatedBalance += balances.DelegatedBalance
			result.OverstakedBalance += balances.OverstakedBalance
			result.StakedBalance += balances.StakedBalance
		}
	}
	return result
}

func (s *StoredDelegationState) ToTzktState() *TzktLikeDelegationState {
	delegators := make([]TzktDelegator, 0, len(s.Balances)-1)
	for addr, balances := range s.Balances {
		if addr != s.Delegate.Address {
			delegators = append(delegators, TzktDelegator{
				Address:          addr,
				DelegatedBalance: balances.DelegatedBalance,
				StakedBalance:    balances.StakedBalance - balances.OverstakedBalance,
			})
		}
	}

	ownBalances := s.OwnDelegatedbalance()
	externalBalances := s.ExternalDelegatedBalance()

	result := &TzktLikeDelegationState{
		Cycle:                    s.Cycle,
		OwnDelegatedBalance:      ownBalances.DelegatedBalance,
		OwnStakedBalance:         ownBalances.StakedBalance,
		ExternalDelegatedBalance: externalBalances.DelegatedBalance,
		ExternalStakedBalance:    externalBalances.StakedBalance,
		DelegatorsCount:          len(delegators),
		Delegators:               delegators,
	}
	return result
}

func CreateStoredDelegationStateFromDelegationState(state *common.DelegationState) *StoredDelegationState {
	return &StoredDelegationState{
		Delegate: Address{state.Baker},
		Cycle:    state.Cycle,
		Status:   DelegationStateStatusOk,
		Balances: DelegationStateBalances(state.DelegatorBalances()),
	}
}
