package core

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tez-capital/ogun/constants"
	"github.com/tez-capital/ogun/test"
	"github.com/trilitech/tzgo/tezos"
)

func getTransport() *test.TestTransport {
	transport, err := test.NewTestTransport(http.DefaultTransport, "../test/data/745", "../test/data/745.zip")
	if err != nil {
		panic(err)
	}
	return transport
}

func TestGetActiveDelegates(t *testing.T) {
	assert := assert.New(t)

	collector, err := NewDefaultRpcCollector("https://eu.rpc.tez.capital/", getTransport())
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, 745)
	assert.Nil(err)
	assert.Equal(354, len(delegates))
}

func TestGetDelegationState(t *testing.T) {
	assert := assert.New(t)

	cycle := int64(745)

	collector, err := NewDefaultRpcCollector("https://eu.rpc.tez.capital/", getTransport())
	assert.Nil(err)

	delegates, err := collector.GetActiveDelegatesFromCycle(defaultCtx, cycle)
	assert.Nil(err)

	channels := make(chan error, len(delegates))
	var wg sync.WaitGroup
	for _, addr := range delegates {
		wg.Add(1)

		go func(addr tezos.Address) {
			defer wg.Done()
			delegate, err := collector.GetDelegateFromCycle(defaultCtx, cycle, addr)
			if err != nil {
				channels <- err
				return
			}

			_, err = collector.GetDelegationState(defaultCtx, delegate)
			if err != nil && err != constants.ErrDelegateHasNoMinimumDelegatedBalance {
				channels <- err
				return
			}
			channels <- nil
		}(addr)
	}
	wg.Wait()
	for i := 0; i < len(delegates); i++ {
		assert.Nil(<-channels)
	}
}
