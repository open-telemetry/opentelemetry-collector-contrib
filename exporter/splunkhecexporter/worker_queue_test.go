package splunkhecexporter

import (
	"fmt"
	"sync"
)

type mockWorkerQueue struct {
	produceResult []error
	current       int
	mu            *sync.Mutex
}

func newMockWorkerQueue(result ...error) *mockWorkerQueue {
	return &mockWorkerQueue{
		produceResult: result,
		current:       0,
		mu:            &sync.Mutex{},
	}
}

func (mock *mockWorkerQueue) Start() error {
	return nil
}

func (mock *mockWorkerQueue) Stop() error {
	return nil
}

func (mock *mockWorkerQueue) Produce(*bufferState, map[string]string) error {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	mock.current++
	if mock.current > len(mock.produceResult) {
		panic(fmt.Sprintf("mockWorkerQueue.Produce was called more than %d times", len(mock.produceResult)))
	}
	return mock.produceResult[mock.current-1]
}

var _ WorkerQueue = &mockWorkerQueue{}
