package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

func TestOverstake(t *testing.T) {
	assert := assert.New(t)

	baker := tezos.MustParseAddress("tz1KqTpEZ7Yob7QbPE4Hy4Wo8fHG8LhKxZSx")

	s := NewDelegationState(&rpc.Delegate{
		Delegate: baker,
	}, 745)

	s.Parameters = &StakingParameters{
		LimitOfStakingOverBakingMillionth: 0,
	}

	s.AddBalance(tezos.MustParseAddress("tz1KqTpEZ7Yob7QbPE4Hy4Wo8fHG8LhKxZSx"), DelegationStateBalanceInfo{
		Balance:         1000000000,
		StakedBalance:   1000,
		UnstakedBalance: 0,
		Baker:           baker,
	})

	delegator := tezos.MustParseAddress("tz1P6WKJu2rcbxKiKRZHKQKmKrpC9TfW1AwM")

	s.AddBalance(delegator, DelegationStateBalanceInfo{
		Balance:         1000000000,
		StakedBalance:   1000,
		UnstakedBalance: 0,
		Baker:           baker,
	})

	assert.Equal(int64(1), s.overstakeFactor().Div64(OVERSTAKE_PRECISION).Int64())

	s.Parameters = &StakingParameters{
		LimitOfStakingOverBakingMillionth: 1000000,
	}

	assert.Equal(int64(0), s.overstakeFactor().Div64(OVERSTAKE_PRECISION).Int64())

	s.Parameters = &StakingParameters{
		LimitOfStakingOverBakingMillionth: 500000,
	}

	assert.Equal(int64(500000), s.overstakeFactor().Int64())
	assert.Equal(int64(500), s.GetDelegatorAndBakerBalances()[delegator].OverstakedBalance)

	delegator2 := tezos.MustParseAddress("tz1S5WxdZR5f9NzsPXhr7L9L1vrEb5spZFur")

	s.AddBalance(delegator2, DelegationStateBalanceInfo{
		Balance:         1000000000,
		StakedBalance:   1000,
		UnstakedBalance: 0,
		Baker:           baker,
	})

	assert.Equal(int64(750000), s.overstakeFactor().Int64())
	assert.Equal(int64(750), s.GetDelegatorAndBakerBalances()[delegator].OverstakedBalance)
	assert.Equal(int64(750), s.GetDelegatorAndBakerBalances()[delegator2].OverstakedBalance)
	assert.Equal(int64(1000000000+750), s.GetDelegatorAndBakerBalances()[delegator].DelegatedBalance)
	assert.Equal(int64(1000000000+750), s.GetDelegatorAndBakerBalances()[delegator2].DelegatedBalance)
	assert.Equal(int64(1000), s.GetDelegatorAndBakerBalances()[delegator].StakedBalance)
	assert.Equal(int64(1000), s.GetDelegatorAndBakerBalances()[delegator2].StakedBalance)
}
