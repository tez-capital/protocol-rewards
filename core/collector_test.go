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

func getTransport() *test.TestTransport {
	transport, err := test.NewTestTransport(http.DefaultTransport, "../test/data/745", "../test/data/745.squashfs")
	if err != nil {
		panic(err)
	}
	return transport
}

func TestGetActiveDelegates(t *testing.T) {
	assert := assert.New(t)

	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport())
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, 745)
	assert.Nil(err)
	assert.Equal(354, len(delegates))
}

func TestGetDelegationStateNoStaking(t *testing.T) {
	assert := assert.New(t)

	cycle := int64(745)

	debug.SetMaxThreads(1000000)

	collector, err := newRpcCollector(defaultCtx, []string{"https://eu.rpc.tez.capital/", "https://rpc.tzkt.io/mainnet/"}, getTransport())
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	channels := make(chan error, len(delegates))
	var wg sync.WaitGroup
	wg.Add(len(delegates))
	for _, addr := range delegates {
		go func(addr tezos.Address) {
			defer wg.Done()
			delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
			if err != nil {
				channels <- err
				return
			}

			_, err = collector.GetDelegationState(defaultCtx, delegate, 745)
			if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
				channels <- err
				return
			}
			channels <- nil
		}(addr)
	}
	wg.Wait()
	for i := 0; i < len(delegates); i++ {
		if err := <-channels; err != nil {
			fmt.Println(delegates[i].String())
		}
		assert.Nil(<-channels)
	}
}
