package core

import (
	"context"
	"sync"

	"github.com/samber/lo"
)

type Job[T any] func(ctx context.Context, item T, mtx *sync.RWMutex) (cancel bool)

func taskRunner[T any](ctx context.Context, taskSource <-chan T, job Job[T], mtx *sync.RWMutex, cancelFn func()) {
	var task T
	var ok bool

	for {
		select { // check if context is done
		case <-ctx.Done():
			return
		case task, ok = <-taskSource:
			if !ok {
				return
			}
		}

		if shouldCancel := job(ctx, task, mtx); shouldCancel {
			cancelFn()
			return
		}
	}
}

func runInParallel[T any](ctx context.Context, collection []T, parallelInstances int, f Job[T]) error {
	taskQueue := lo.SliceToChannel(parallelInstances, collection)
	mtx := sync.RWMutex{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(parallelInstances)
	for i := 0; i < parallelInstances; i++ {
		go func() {
			defer wg.Done()
			taskRunner(ctx, taskQueue, f, &mtx, cancel)
		}()
	}
	wg.Wait()

	return nil
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
