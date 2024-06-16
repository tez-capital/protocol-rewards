package store

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/tez-capital/ogun/common"
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

type StoredDelegationState struct {
	Delegate []byte                  `json:"delegate" grom:"primaryKey"`
	Cycle    int64                   `json:"cycle" grom:"primarykey"`
	Status   DelegationStateStatus   `json:"status" grom:"primarykey"`
	Balances DelegationStateBalances `json:"balances" grom:"type:jsonb;default:'{}'"`
}

func CreateStoredDelegationStateFromDelegationState(state *common.DelegationState) *StoredDelegationState {
	return &StoredDelegationState{
		Delegate: state.Baker.Encode(),
		Cycle:    state.Cycle,
		Status:   DelegationStateStatusOk,
		Balances: DelegationStateBalances(state.DelegatorDelegatedBalances()),
	}
}

type FetchedCycles struct {
	Cycle                 int64 `json:"cycle" gorm:"primaryKey"`
	StateCount            int   `json:"state_count"`
	StateWithBalanceCount int   `json:"state_with_balance_count"`
}
