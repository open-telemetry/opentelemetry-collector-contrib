// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package threadpool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testRequest struct {
	i int
}

type testCase struct {
	size     int
	received []testRequest
	lock     sync.Mutex
}

func (c *testCase) testWorker(_ context.Context, queue chan testRequest) {
	for {
		select {
		case req, ok := <-queue:
			if !ok {
				return
			}
			c.lock.Lock()
			c.received = append(c.received, req)
			c.lock.Unlock()

			// pretend to do some work
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// Test correctness of threadpool
func TestThreadpool(t *testing.T) {
	testCases := []*testCase{
		{
			size: 100,
		},
		{
			size: 200,
		},
		{
			size: 1000,
		},
		{
			size: 2000,
		},
	}
	for _, c := range testCases {
		pool := NewPool(c.size, c.testWorker)
		pool.StartConsumers(context.Background())
		sent := make([]testRequest, 0)
		// wait for some time
		ticker := time.NewTicker(100 * time.Millisecond)
	OUTER:
		for i := 0; ; i++ {
			select {
			case <-ticker.C:
				break OUTER
			default:
				item := testRequest{i: i}
				pool.Submit(item)
				sent = append(sent, item)
			}
		}
		pool.StopConsumers()
		require.ElementsMatch(t, c.received, sent)
	}
}
