package core

import (
	"context"
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

	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport("../test/data/745"))
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
	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport("../test/data/745"))
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, 745)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)

	// cycle 746
	cycle = int64(745)
	collector, err = newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport("../test/data/746"))
	assert.Nil(err)

	delegates, err = collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	err = runInParallel(defaultCtx, delegates, constants.OGUN_DELEGATE_FETCH_BATCH_SIZE, func(ctx context.Context, addr tezos.Address, mtx *sync.RWMutex) bool {
		delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
		if err != nil {
			assert.Nil(err)
			return true
		}

		_, err = collector.GetDelegationState(defaultCtx, delegate, 745)
		if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
			assert.Nil(err)
			return true
		}
		return false
	})
	assert.Nil(err)
}
