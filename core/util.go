package core

import (
	"context"
	"sync"

	"github.com/samber/lo"
)

func runInBatches[T any](ctx context.Context, collection []T, bufferSize int, f func(ctx context.Context, item T, mtx *sync.Mutex) (cancel bool)) error {
	ch := lo.SliceToChannel(bufferSize, collection)
	mtx := sync.Mutex{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(collection))
	for {
		var batchWg sync.WaitGroup
		items, _, _, ok := lo.Buffer(ch, bufferSize)
		batchWg.Add(len(items))
		for _, item := range items {
			go func(item T) {
				defer wg.Done()
				defer batchWg.Done()

				if shouldCancel := f(ctx, item, &mtx); shouldCancel {
					cancel()
				}
			}(item)
		}
		batchWg.Wait()
		if !ok {
			break
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
