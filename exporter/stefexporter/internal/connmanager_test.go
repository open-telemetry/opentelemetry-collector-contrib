// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockConnCreator struct {
	conns    []*mockConn
	connsMux sync.Mutex
}

func (m *mockConnCreator) Create(context.Context) (Conn, error) {
	conn := &mockConn{}
	m.connsMux.Lock()
	m.conns = append(m.conns, conn)
	m.connsMux.Unlock()
	return conn, nil
}

type mockConn struct {
	closed  bool
	flushed bool
	mux     sync.RWMutex
}

func (m *mockConn) Close(context.Context) error {
	m.mux.Lock()
	m.closed = true
	m.mux.Unlock()
	return nil
}

func (m *mockConn) Flush(context.Context) error {
	m.mux.Lock()
	m.flushed = true
	m.mux.Unlock()
	return nil
}

func (m *mockConn) Flushed() bool {
	m.mux.RLock()
	defer m.mux.RUnlock()
	return m.flushed
}

func (m *mockConn) Closed() bool {
	m.mux.RLock()
	defer m.mux.RUnlock()
	return m.closed
}

func (m *mockConn) MarkNotFlushed() {
	m.mux.Lock()
	m.flushed = false
	m.mux.Unlock()
}

const (
	flushPeriod     = time.Hour
	reconnectPeriod = 2 * time.Hour
)

type testDef struct {
	clock         clockwork.Clock
	test          func(cm *ConnManager)
	afterStopFunc func(creator *mockConnCreator, connCount uint)
}

func doTest(t *testing.T, test testDef) {
	for connCount := uint(1); connCount <= 2; connCount++ {
		connCreator := &mockConnCreator{}

		set := ConnManagerSettings{
			Logger:          zap.NewNop(),
			Creator:         connCreator,
			TargetConnCount: connCount,
			FlushPeriod:     flushPeriod,
			ReconnectPeriod: reconnectPeriod,
		}
		cm, err := NewConnManager(set)
		require.NoError(t, err)

		if test.clock != nil {
			cm.clock = test.clock
		}

		cm.Start()

		test.test(cm)

		assert.NoError(t, cm.Stop(context.Background()))

		for _, conn := range connCreator.conns {
			require.True(t, conn.closed)
		}

		if test.afterStopFunc != nil {
			test.afterStopFunc(connCreator, connCount)
		}
	}
}

func TestConnManagerStartStop(t *testing.T) {
	doTest(t, testDef{test: func(*ConnManager) {}})
}

func TestConnManagerAcquireRelease(t *testing.T) {
	doTest(
		t, testDef{
			test: func(cm *ConnManager) {
				conn, err := cm.Acquire(context.Background())
				require.NoError(t, err)
				require.NotNil(t, conn)
				cm.Release(context.Background(), conn)
			},
		},
	)
}

func TestConnManagerAcquireReleaseMany(t *testing.T) {
	var conns []*ManagedConn
	doTest(
		t, testDef{
			test: func(cm *ConnManager) {
				conns = nil
				for i := uint(0); i < cm.targetConnCount; i++ {
					conn, err := cm.Acquire(context.Background())
					require.NoError(t, err)
					require.NotNil(t, conn)
					conns = append(conns, conn)
				}
				for _, conn := range conns {
					cm.Release(context.Background(), conn)
				}
			},
			afterStopFunc: func(_ *mockConnCreator, _ uint) {
				// Verify that Stop() flushes all connections.
				for _, conn := range conns {
					require.True(t, conn.conn.(*mockConn).flushed)
				}
			},
		},
	)
}

func TestConnManagerAcquireDiscardAcquire(t *testing.T) {
	var conns []*ManagedConn
	doTest(
		t, testDef{
			test: func(cm *ConnManager) {
				conns = nil

				conn, err := cm.Acquire(context.Background())
				require.NoError(t, err)

				// Discard the connection. This should restart in a replacement
				// connection creation.
				cm.DiscardAndClose(conn)

				// Make sure we ask for maximum number of possible connections
				// so that we guarantee the discarded connection is replaced.
				for i := uint(0); i < cm.targetConnCount; i++ {
					conn, err := cm.Acquire(context.Background())
					require.NoError(t, err)
					require.NotNil(t, conn)
					conns = append(conns, conn)
				}
				for _, conn := range conns {
					cm.Release(context.Background(), conn)
				}
			},
			afterStopFunc: func(creator *mockConnCreator, connCount uint) {
				// Make sure one more connection is created because we discarded one.
				require.Len(t, creator.conns, int(connCount+1))
			},
		},
	)
}

func TestConnManagerAcquireReleaseConcurrent(t *testing.T) {
	doTest(
		t, testDef{
			test: func(cm *ConnManager) {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						conn, err := cm.Acquire(context.Background())
						if err != nil {
							return
						}
						cm.Release(context.Background(), conn)
					}()
				}
				wg.Wait()
			},
		},
	)
}

func TestConnManagerAcquireDiscard(t *testing.T) {
	doTest(
		t, testDef{
			test: func(cm *ConnManager) {
				conn, err := cm.Acquire(context.Background())
				require.NoError(t, err)
				cm.DiscardAndClose(conn)
			},
		},
	)
}

func TestConnManagerAcquireDiscardConcurrent(t *testing.T) {
	doTest(
		t, testDef{
			test: func(cm *ConnManager) {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						conn, err := cm.Acquire(context.Background())
						if err != nil {
							return
						}
						cm.DiscardAndClose(conn)
					}()
				}
				wg.Wait()
			},
			afterStopFunc: func(creator *mockConnCreator, _ uint) {
				// Verify that Stop() did not flush any connections since we used DiscardAndClose().
				for _, conn := range creator.conns {
					require.False(t, conn.flushed)
				}
			},
		},
	)
}

func TestConnManagerFlush(t *testing.T) {
	doTest(
		t, testDef{
			clock: clockwork.NewFakeClock(),
			test: func(cm *ConnManager) {
				// First test the scenario where the flush is done immediately
				// by Release().
				conn, err := cm.Acquire(context.Background())
				require.NoError(t, err)

				// Make sure the flush is not done yet.
				assert.False(t, conn.conn.(*mockConn).Flushed())

				// Advance the clock so that Release() trigger flush.
				cm.clock.(*clockwork.FakeClock).Advance(flushPeriod)
				cm.Release(context.Background(), conn)

				// Make sure the flush is done.
				assert.Eventually(
					t, func() bool {
						return conn.conn.(*mockConn).Flushed()
					},
					5*time.Second,
					time.Millisecond*10,
				)

				// Try one more time, but now with flushing happening after Release()
				conn, err = cm.Acquire(context.Background())
				require.NoError(t, err)

				// It is the same connection which was already flushed.
				// Mark it as not flushed so we can test the subsequent flush.
				conn.conn.(*mockConn).MarkNotFlushed()

				// Make sure the flush is not done yet.
				assert.False(t, conn.conn.(*mockConn).Flushed())

				// Release the connection first.
				cm.Release(context.Background(), conn)

				// Advance the clock so that the connection is flush in the background.
				cm.clock.(*clockwork.FakeClock).Advance(flushPeriod)

				// Make sure the flush is done.
				assert.Eventually(
					t, func() bool {
						return conn.conn.(*mockConn).Flushed()
					},
					5*time.Second,
					time.Millisecond*10,
				)
			},
		},
	)
}

func TestConnManagerReconnector(t *testing.T) {
	doTest(
		t, testDef{
			clock: clockwork.NewFakeClock(),
			test: func(cm *ConnManager) {
				conn, err := cm.Acquire(context.Background())
				require.NoError(t, err)
				require.NotNil(t, conn)
				cm.Release(context.Background(), conn)

				assert.Eventually(
					t, func() bool {
						// Advance the clock to make sure durationLimiter progresses.
						cm.clock.(*clockwork.FakeClock).Advance(reconnectPeriod)

						// This connection should be flushed because Release()
						// marks it for flushing and durationLimiter flushes it.
						return conn.conn.(*mockConn).Flushed() && conn.conn.(*mockConn).Closed()
					}, 5*time.Second, time.Millisecond*10,
				)
			},
		},
	)
}
