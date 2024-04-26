// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

type mockTraceListener struct {
	started   bool
	closed    bool
	startWait *sync.WaitGroup
	closeWait *sync.WaitGroup
}

func (m *mockTraceListener) ListenAndServe(handler fdbTraceHandler, maxPacketSize int) error {
	m.started = true
	m.startWait.Done()
	return nil
}

func (m *mockTraceListener) Close() error {
	m.closed = true
	m.closeWait.Done()
	return nil
}

func TestStartsTraceListener(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	receiver := &foundationDBReceiver{
		config:   &Config{},
		listener: &mockTraceListener{startWait: wg},
		logger:   zap.NewNop(),
		consumer: &MockTraceConsumer{},
		handler:  &openTelemetryHandler{},
	}
	err := receiver.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	assert.True(t, waitTimeout(wg, time.Second*5))
	assert.True(t, receiver.listener.(*mockTraceListener).started)
	assert.False(t, receiver.listener.(*mockTraceListener).closed)
}

func TestClosesWhenContextCanceled(t *testing.T) {
	startWait := &sync.WaitGroup{}
	startWait.Add(1)
	closeWait := &sync.WaitGroup{}
	closeWait.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	receiver := &foundationDBReceiver{
		config:   &Config{},
		listener: &mockTraceListener{startWait: startWait, closeWait: closeWait},
		logger:   zap.NewNop(),
		consumer: &MockTraceConsumer{},
		handler:  &openTelemetryHandler{},
	}
	err := receiver.Start(ctx, componenttest.NewNopHost())
	assert.NoError(t, err)
	assert.True(t, waitTimeout(startWait, time.Second*5))
	assert.True(t, receiver.listener.(*mockTraceListener).started)
	cancel()
	assert.True(t, waitTimeout(closeWait, time.Second*5))
	assert.True(t, receiver.listener.(*mockTraceListener).closed)
}

func TestShutdownCloses(t *testing.T) {
	startWait := &sync.WaitGroup{}
	startWait.Add(1)
	closeWait := &sync.WaitGroup{}
	closeWait.Add(1)
	receiver := &foundationDBReceiver{
		config:   &Config{},
		listener: &mockTraceListener{startWait: startWait, closeWait: closeWait},
		logger:   zap.NewNop(),
		consumer: &MockTraceConsumer{},
		handler:  &openTelemetryHandler{},
	}
	err := receiver.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	assert.True(t, waitTimeout(startWait, time.Second*5))
	assert.True(t, receiver.listener.(*mockTraceListener).started)
	err = receiver.Shutdown(context.Background())
	assert.True(t, waitTimeout(closeWait, time.Second*5))
	assert.True(t, receiver.listener.(*mockTraceListener).closed)
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return true
	case <-time.After(timeout):
		return false
	}
}
