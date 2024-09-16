// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

func Test_Server_ListenAndServe(t *testing.T) {
	tests := []struct {
		name          string
		buildServerFn func(addr string) (Server, error)
		buildClientFn func(addr string) (*client.Graphite, error)
	}{
		{
			name: "tcp",
			buildServerFn: func(addr string) (Server, error) {
				return NewTCPServer(addr, 1*time.Second)
			},
			buildClientFn: func(addr string) (*client.Graphite, error) {
				return client.NewGraphite(client.TCP, addr)
			},
		},
		{
			name:          "udp",
			buildServerFn: NewUDPServer,
			buildClientFn: func(addr string) (*client.Graphite, error) {
				return client.NewGraphite(client.UDP, addr)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := testutil.GetAvailableLocalNetworkAddress(t, tt.name)

			svr, err := tt.buildServerFn(addr)
			require.NoError(t, err)
			require.NotNil(t, svr)

			mc := new(consumertest.MetricsSink)
			p, err := (&protocol.PlaintextConfig{}).BuildParser()
			require.NoError(t, err)
			mr := &mockReporter{}
			mr.wgMetricsProcessed.Add(1)

			wgListenAndServe := sync.WaitGroup{}
			wgListenAndServe.Add(1)
			go func() {
				defer wgListenAndServe.Done()
				assert.Error(t, svr.ListenAndServe(p, mc, mr))
			}()

			runtime.Gosched()

			gc, err := tt.buildClientFn(addr)
			require.NoError(t, err)
			require.NotNil(t, gc)

			ts := time.Date(2020, 2, 20, 20, 20, 20, 20, time.UTC)
			err = gc.SendMetric(client.Metric{
				Name: "test.metric", Value: 1, Timestamp: ts})
			assert.NoError(t, err)
			runtime.Gosched()

			err = gc.Disconnect()
			assert.NoError(t, err)

			mr.wgMetricsProcessed.Wait()

			// Keep trying until we're timed out or got a result
			assert.Eventually(t, func() bool {
				mdd := mc.AllMetrics()
				return len(mdd) > 0
			}, 10*time.Second, 500*time.Millisecond)

			err = svr.Close()
			assert.NoError(t, err)

			wgListenAndServe.Wait()

			mdd := mc.AllMetrics()
			require.Len(t, mdd, 1)
			require.Equal(t, 1, mdd[0].MetricCount())
			assert.Equal(t, "test.metric", mdd[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
		})
	}
}

// mockReporter provides a Reporter that provides some useful functionalities for
// tests (eg.: wait for certain number of messages).
type mockReporter struct {
	wgMetricsProcessed sync.WaitGroup
}

func (m *mockReporter) OnDataReceived(ctx context.Context) context.Context {
	return ctx
}

func (m *mockReporter) OnTranslationError(context.Context, error) {}

func (m *mockReporter) OnMetricsProcessed(context.Context, int, error) {
	m.wgMetricsProcessed.Done()
}

func (m *mockReporter) OnDebugf(string, ...any) {}
