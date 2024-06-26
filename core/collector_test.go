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

	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, 745)
	assert.Nil(err)
	assert.Equal(354, len(delegates))
}

func TestGetDelegationStateNoStaking(t *testing.T) {
	assert := assert.New(t)
	debug.SetMaxThreads(1000000)

	// cycle 745
	cycle := int64(745)
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)

	// cycle 746
	cycle = int64(746)
	collector, err = newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err = collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle)
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
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle)
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
	/*
		- tz1S5WxdZR5f9NzsPXhr7L9L1vrEb5spZFur
		- tz1eu3mkvEjzPgGoRMuKY7EHHtSwz88VxS31
		- tz3LV9aGKHDnAZHCtC9SjNtTrKRu678FqSki
		- tz1aKxnrzx5PXZJe7unufEswVRCMU9yafmfb
		- tz1ZgkTFmiwddPXGbs4yc6NWdH4gELW7wsnv
	*/
	cycle := int64(749)
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport(fmt.Sprintf("../test/data/%d", cycle)))
	assert.Nil(err)

	delegates := []tezos.Address{
		tezos.MustParseAddress("tz1S5WxdZR5f9NzsPXhr7L9L1vrEb5spZFur"),
		tezos.MustParseAddress("tz1eu3mkvEjzPgGoRMuKY7EHHtSwz88VxS31"),
		tezos.MustParseAddress("tz3LV9aGKHDnAZHCtC9SjNtTrKRu678FqSki"),
		tezos.MustParseAddress("tz1aKxnrzx5PXZJe7unufEswVRCMU9yafmfb"),
		tezos.MustParseAddress("tz1ZgkTFmiwddPXGbs4yc6NWdH4gELW7wsnv"),
	}

	err = runInParallel(defaultCtx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, cycle)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)
}
