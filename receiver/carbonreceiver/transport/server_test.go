// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport/client"
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
			mr := NewMockReporter(1)

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

			mr.WaitAllOnMetricsProcessedCalls()

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
			_, _, metrics := internaldata.ResourceMetricsToOC(mdd[0].ResourceMetrics().At(0))
			require.Len(t, metrics, 1)
			assert.Equal(t, "test.metric", metrics[0].GetMetricDescriptor().GetName())
		})
	}
}
