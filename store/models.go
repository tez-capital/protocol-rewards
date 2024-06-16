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
}

type TzktLikeDelegationState struct {
	Cycle                    int64           `json:"cycle"`
	OwnDelegatedBalance      int64           `json:"ownDelegatedBalance"`
	ExternalDelegatedBalance int64           `json:"externalDelegatedBalance"`
	DelegatorsCount          int             `json:"delegatorsCount"`
	Delegators               []TzktDelegator `json:"delegators"`
}

type StoredDelegationState struct {
	Delegate Address                 `json:"delegate" gorm:"primaryKey"`
	Cycle    int64                   `json:"cycle" gorm:"primaryKey"`
	Status   DelegationStateStatus   `json:"status"`
	Balances DelegationStateBalances `json:"balances" gorm:"type:jsonb;default:'{}'"`
}

func (s *StoredDelegationState) OwnDelegatedbalance() int64 {
	return s.Balances[s.Delegate.Address]
}

func (s *StoredDelegationState) ExternalDelegatedBalance() int64 {
	var result int64
	for addr, balance := range s.Balances {
		if addr != s.Delegate.Address {
			result += balance
		}
	}
	return result
}

func (s *StoredDelegationState) ToTzktState() *TzktLikeDelegationState {
	delegators := make([]TzktDelegator, 0, len(s.Balances)-1)
	for addr, balance := range s.Balances {
		if addr != s.Delegate.Address {
			delegators = append(delegators, TzktDelegator{
				Address:          addr,
				DelegatedBalance: balance,
			})
		}
	}

	result := &TzktLikeDelegationState{
		Cycle:                    s.Cycle,
		OwnDelegatedBalance:      s.OwnDelegatedbalance(),
		ExternalDelegatedBalance: s.ExternalDelegatedBalance(),
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
		Balances: DelegationStateBalances(state.DelegatorDelegatedBalances()),
	}
}
