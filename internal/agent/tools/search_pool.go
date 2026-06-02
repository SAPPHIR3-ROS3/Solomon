package tools

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

const maxSearchWorkers = 8

func searchWorkerCount() int {
	n := maxSearchWorkers
	if c := runtime.NumCPU(); c < n {
		n = c
	}
	if n < 1 {
		n = 1
	}
	return n
}

type headLimiter struct {
	max   int32
	count atomic.Int32
}

func newHeadLimiter(max int) *headLimiter {
	if max <= 0 {
		return nil
	}
	return &headLimiter{max: int32(max)}
}

func (h *headLimiter) allow() bool {
	if h == nil {
		return true
	}
	for {
		cur := h.count.Load()
		if cur >= h.max {
			return false
		}
		if h.count.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}

func runParallel[T any](ctx context.Context, in <-chan T, workers int, fn func(context.Context, T) error) error {
	if workers < 1 {
		workers = 1
	}
	wg := sync.WaitGroup{}
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-in:
					if !ok {
						return
					}
					if err := fn(ctx, item); err != nil {
						errCh <- err
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
