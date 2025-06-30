// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSync2Async(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	var globalID DataID
	jobs := make(
		chan struct {
			data       any
			dataID     DataID
			resultChan ResultChan
		},
	)

	var asyncMux sync.Mutex

	async := func(
		_ context.Context,
		data any,
		resultChan ResultChan,
	) (DataID, error) {
		asyncMux.Lock()
		globalID++
		ackID := globalID
		asyncMux.Unlock()

		jobs <- struct {
			data       any
			dataID     DataID
			resultChan ResultChan
		}{
			data:       data,
			dataID:     ackID,
			resultChan: resultChan,
		}

		return ackID, nil
	}

	// Perform the jobs in a separate goroutine.
	go func() {
		for job := range jobs {
			var err error
			if job.data.(int)%10 == 0 {
				// Make it an error once in a while.
				err = errors.New("some error")
			}
			job.resultChan <- AsyncResult{DataID: job.dataID, Err: err}
		}
	}()

	const syncProducers = 100
	s2a := NewSync2Async(logger, syncProducers, async)

	ctx := context.Background()
	var wg sync.WaitGroup
	const countPerProducer = 100
	const totalCount = syncProducers * countPerProducer

	expectedErr := false
	var unexpectedErr error

	for i := 0; i < syncProducers; i++ {
		wg.Add(1)
		go func(data int) {
			defer wg.Done()
			for j := 0; j < countPerProducer; j++ {
				err := s2a.DoSync(ctx, data)
				if data%10 == 0 {
					// Must be an error.
					if err == nil {
						expectedErr = true
						return
					}
				} else {
					// Must not be an error.
					if err != nil {
						unexpectedErr = err
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()
	close(jobs)

	if expectedErr {
		t.Fatal("Expected error but got nil")
	}
	require.NoError(t, unexpectedErr)

	require.EqualValues(t, totalCount, globalID)
}
