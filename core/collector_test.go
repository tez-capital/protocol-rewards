package core

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tez-capital/ogun/constants"
	"github.com/tez-capital/ogun/test"
	"github.com/trilitech/tzgo/rpc"
	"github.com/trilitech/tzgo/tezos"
)

var (
	defaultCtx context.Context = context.Background()
)

func getTransport(path string) *test.TestTransport {
	transport, err := test.NewTestTransport(http.DefaultTransport, path, path+".gob.lz4")
	if err != nil {
		panic(err)
	}
	return transport
}

func TestGetActiveDelegates(t *testing.T) {
	assert := assert.New(t)

	cycle := 745
	lastBlockInTheCycle := rpc.BlockLevel(5799936)
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, []string{"https://api.tzkt.io/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, lastBlockInTheCycle)
	assert.Nil(err)
	assert.Equal(354, len(delegates))
}

func TestGetDelegationStateNoStaking(t *testing.T) {
	assert := assert.New(t)
	debug.SetMaxThreads(1000000)

	// cycle 745
	cycle := int64(745)
	lastBlockInTheCycle := rpc.BlockLevel(5799936)
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, []string{"https://api.tzkt.io/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, lastBlockInTheCycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, 100, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, lastBlockInTheCycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle, lastBlockInTheCycle)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)

	// cycle 746
	cycle = int64(746)
	lastBlockInTheCycle = rpc.BlockLevel(5824512)
	collector, err = newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, []string{"https://api.tzkt.io/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err = collector.GetActiveDelegatesFromCycle(defaultCtx, lastBlockInTheCycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, 100, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, lastBlockInTheCycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle, lastBlockInTheCycle)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)
}

func TestGetDelegationState(t *testing.T) {
	assert := assert.New(t)
	debug.SetMaxThreads(1000000)

	// cycle 748
	cycle := int64(748)
	lastBlockInTheCycle := rpc.BlockLevel(5873664)
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, []string{"https://api.tzkt.io/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, lastBlockInTheCycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, 100, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, lastBlockInTheCycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle, lastBlockInTheCycle)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)
}

func TestCycle749RaceConditions(t *testing.T) {
	assert := assert.New(t)
	debug.SetMaxThreads(1000000)

	cycle := int64(749)
	lastBlockInTheCycle := rpc.BlockLevel(5898240)
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, []string{"https://api.tzkt.io/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates := []tezos.Address{
		tezos.MustParseAddress("tz1S5WxdZR5f9NzsPXhr7L9L1vrEb5spZFur"),
		tezos.MustParseAddress("tz1eu3mkvEjzPgGoRMuKY7EHHtSwz88VxS31"),
		tezos.MustParseAddress("tz3LV9aGKHDnAZHCtC9SjNtTrKRu678FqSki"),
		tezos.MustParseAddress("tz1aKxnrzx5PXZJe7unufEswVRCMU9yafmfb"),
		tezos.MustParseAddress("tz1ZgkTFmiwddPXGbs4yc6NWdH4gELW7wsnv"),
		tezos.MustParseAddress("tz1NuAqi3T35CPZV7tQu94wa3urCCzJrV7zc"),
		tezos.MustParseAddress("tz3Uzceas5ZauAh47FkKEVLupFoXstWq7MbX"),
	}

	err = runInParallel(defaultCtx, delegates, 100, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, lastBlockInTheCycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle, lastBlockInTheCycle)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			fmt.Println(delegate.Delegate.String())
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)
}
