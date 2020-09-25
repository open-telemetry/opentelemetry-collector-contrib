// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package requests

import (
	"context"
	"net/http"

	"go.uber.org/atomic"
)

// HTTPSender interface is used as callback provided to HTTPWorkPool.
type HTTPSender interface {
	SendRequest(req *http.Request) error
}

// HTTPWorkPool manages a pool of workers for sending HTTP requests in parallel.
type HTTPWorkPool struct {
	sender                 HTTPSender
	ctx                    context.Context
	requests               chan *http.Request
	workPool               chan struct{}
	RunningWorkers         atomic.Int64
	TotalRequestsWaiting   atomic.Int64
	TotalRequestsStarted   atomic.Int64
	TotalRequestsCompleted atomic.Int64
	TotalRequestsFailed    atomic.Int64
}

// NewHTTPWorkPool creates a new workpool with the lifetime of the given context. A maximum number of workers
// specified with workerCount are started. The provided sender is used to process requests.
func NewHTTPWorkPool(ctx context.Context, workerCount int, sender HTTPSender) *HTTPWorkPool {
	return &HTTPWorkPool{
		sender: sender,
		ctx:    ctx,
		// Unbuffered so that it blocks clients
		requests: make(chan *http.Request),
		// Limits size of work pool.
		workPool: make(chan struct{}, workerCount),
	}
}

// Send the given request object. Will block unless:
//
// 1. A worker is available.
// 2. Workpool has been stopped.
// 3. Request context is cancelled.
func (rs *HTTPWorkPool) Send(req *http.Request) {
	// Slight optimization to avoid spinning up unnecessary workers if there
	// aren't ever that many updates. Once workers start, they remain for
	// the life of the workpool.
	select {
	// Worker is immediately available.
	case rs.requests <- req:
		return
	default:
		// Worker is not immediately available.
		select {
		// Start a new worker if the work pool isn't at max capacity.
		case rs.workPool <- struct{}{}:
			rs.startWorker()
		default:
		}

		rs.TotalRequestsWaiting.Inc()
		// Block until we can get through a request or context is cancelled.
		select {
		case <-req.Context().Done():
		case <-rs.ctx.Done():
		case rs.requests <- req:
		}

	}
}

func (rs *HTTPWorkPool) startWorker() {
	go rs.processRequests()
}

func (rs *HTTPWorkPool) processRequests() {
	rs.RunningWorkers.Inc()
	defer rs.RunningWorkers.Dec()
	defer func() {
		// Not strictly necessary since we don't stop workers except on shutdown but
		// will keep len(workPool) usually in line with RunningWorkers.
		<-rs.workPool
	}()

	for {
		select {
		case <-rs.ctx.Done():
			return
		case req := <-rs.requests:
			rs.TotalRequestsStarted.Inc()
			if err := rs.sender.SendRequest(req); err != nil {
				rs.TotalRequestsFailed.Inc()
				continue
			}
			rs.TotalRequestsCompleted.Inc()
		}
	}
}
