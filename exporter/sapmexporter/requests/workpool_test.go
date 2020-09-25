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
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dummyRequest = httptest.NewRequest(http.MethodGet, "http://localhost", nil)

type fakeSender struct {
	err error
}

func (f *fakeSender) SendRequest(req *http.Request) error {
	return f.err
}

func newWorkPool(t *testing.T, count int, sender HTTPSender) *HTTPWorkPool {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewHTTPWorkPool(ctx, count, sender)
}

// shortWait waits 1 second for condition to evaluate to true.
func shortWait(t *testing.T, eventually func(t require.TestingT, condition func() bool, waitFor time.Duration, tick time.Duration, msgAndArgs ...interface{}), f func() bool) {
	t.Helper()
	eventually(t, f, 1*time.Second, 1*time.Millisecond)
}

func TestNewWorkPool(t *testing.T) {
	assert.NotNil(t, NewHTTPWorkPool(context.Background(), 1, nil))
}

func TestNewWorker(t *testing.T) {
	w := newWorkPool(t, 5, &fakeSender{})
	assert.Equal(t, 0, len(w.workPool))
	w.Send(dummyRequest)
	assert.Equal(t, 1, len(w.workPool))
}

func TestExistingWorkerReused(t *testing.T) {
	done := make(chan struct{})
	w := newWorkPool(t, 1, &signaledSender{done: done})
	w.Send(dummyRequest)
	assert.Equal(t, 1, len(w.workPool))
	done <- struct{}{}
	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsCompleted.Load() == 1
	})

	w.Send(dummyRequest)
	done <- struct{}{}

	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsCompleted.Load() == 2
	})

	assert.Equal(t, 1, len(w.workPool))
}

type signaledSender struct {
	done chan struct{}
}

func (s *signaledSender) SendRequest(req *http.Request) error {
	<-s.done
	return nil
}

func TestMultipleWorkersCreated(t *testing.T) {
	done := make(chan struct{})
	w := newWorkPool(t, 5, &signaledSender{done: done})
	w.Send(dummyRequest)
	w.Send(dummyRequest)
	close(done)
	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsCompleted.Load() == 2
	})
	assert.Equal(t, 2, len(w.workPool))
}

func TestWorkPoolAtMaxCapacity(t *testing.T) {
	done := make(chan struct{})
	w := newWorkPool(t, 3, &signaledSender{done: done})
	w.Send(dummyRequest)
	w.Send(dummyRequest)
	w.Send(dummyRequest)
	go func() {
		w.Send(dummyRequest)
	}()
	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsWaiting.Load() == 4
	})
	close(done)
	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsCompleted.Load() == 4
	})
	assert.Equal(t, 3, len(w.workPool))
}

func TestShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	w := NewHTTPWorkPool(ctx, 1, &signaledSender{done: done})
	w.Send(dummyRequest)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		w.Send(dummyRequest)
		wg.Done()
	}()

	cancel()
	wg.Wait()
}

func TestSendReturnsWhenRequestContextIsCancelled(t *testing.T) {
	w := NewHTTPWorkPool(context.Background(), 0, &fakeSender{})
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		w.Send(req)
		wg.Done()
	}()

	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsWaiting.Load() == 1
	})

	cancel()

	wg.Wait()

	assert.Equal(t, int64(0), w.TotalRequestsCompleted.Load())

	require.NoError(t, err)
}

func TestRequestFailed(t *testing.T) {
	w := newWorkPool(t, 1, &fakeSender{err: errors.New("request failed")})
	w.Send(dummyRequest)
	shortWait(t, require.Eventually, func() bool {
		return w.TotalRequestsFailed.Load() == 1
	})
}
