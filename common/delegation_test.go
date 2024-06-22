package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

func TestOverstake(t *testing.T) {
	assert := assert.New(t)

	s := NewDelegationState(&rpc.Delegate{
		Delegate: tezos.MustParseAddress("tz1KqTpEZ7Yob7QbPE4Hy4Wo8fHG8LhKxZSx"),
	}, 745)

	s.Parameters = &StakingParameters{
		LimitOfStakingOverBakingMillionth: 0,
	}

	s.AddBalance(tezos.MustParseAddress("tz1KqTpEZ7Yob7QbPE4Hy4Wo8fHG8LhKxZSx"), DelegationStateBalanceInfo{
		Balance:         1000000000,
		StakedBalance:   1000,
		UnstakedBalance: 0,
	})

	delegator := tezos.MustParseAddress("tz1P6WKJu2rcbxKiKRZHKQKmKrpC9TfW1AwM")

	s.AddBalance(delegator, DelegationStateBalanceInfo{
		Balance:         1000000000,
		StakedBalance:   1000,
		UnstakedBalance: 0,
	})

	assert.True(s.overstakeFactor().Div64(OVERSTAKE_PRECISION).Equal(tezos.NewZ(1)))

	s.Parameters = &StakingParameters{
		LimitOfStakingOverBakingMillionth: 1000000,
	}

	assert.True(s.overstakeFactor().Div64(OVERSTAKE_PRECISION).Equal(tezos.NewZ(0)))

	s.Parameters = &StakingParameters{
		LimitOfStakingOverBakingMillionth: 500000,
	}

	assert.True(s.overstakeFactor().Equal(tezos.NewZ(500000)))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator].OverstakedBalance, int64(500))

	delegator2 := tezos.MustParseAddress("tz1S5WxdZR5f9NzsPXhr7L9L1vrEb5spZFur")

	s.AddBalance(delegator2, DelegationStateBalanceInfo{
		Balance:         1000000000,
		StakedBalance:   1000,
		UnstakedBalance: 0,
	})

	assert.True(s.overstakeFactor().Equal(tezos.NewZ(750000)))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator].OverstakedBalance, int64(750))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator2].OverstakedBalance, int64(750))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator].DelegatedBalance, int64(1000000000+750))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator2].DelegatedBalance, int64(1000000000+750))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator].StakedBalance, int64(1000))
	assert.Equal(s.GetDelegatorAndBakerBalances()[delegator2].StakedBalance, int64(1000))
}
