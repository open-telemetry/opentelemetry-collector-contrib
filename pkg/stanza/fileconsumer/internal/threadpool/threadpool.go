package threadpool

import (
	"context"
	"sync"
)

type Pool[T any] struct {
	jobQueue   chan T
	size       int
	wg         sync.WaitGroup
	workerFunc func(context.Context, chan T)
}

func NewPool[T any](maxBatchFiles int, workerFunc func(context.Context, chan T)) *Pool[T] {
	return &Pool[T]{
		workerFunc: workerFunc,
		size:       maxBatchFiles,
	}
}

// startConsumers starts a given number of goroutines consuming items from the channel
func (pool *Pool[T]) StartConsumers(ctx context.Context) {
	pool.jobQueue = make(chan T, pool.size)
	for i := 0; i < pool.size; i++ {
		pool.wg.Add(1)
		go pool.consume(ctx)
	}
}

// stopConsumers closes the channel created during startConsumers, wait for the consumers to finish execution
// and saves any files left
func (pool *Pool[T]) StopConsumers() {
	if pool.jobQueue != nil {
		close(pool.jobQueue)
	}
	pool.wg.Wait()
}

// Submit queues the request in jobQueue
func (pool *Pool[T]) Submit(req T) {
	pool.jobQueue <- req
}

func (pool *Pool[T]) consume(ctx context.Context) {
	defer pool.wg.Done()
	pool.workerFunc(ctx, pool.jobQueue)
}
