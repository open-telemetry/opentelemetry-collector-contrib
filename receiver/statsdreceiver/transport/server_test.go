// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"net"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport/client"
)

func Test_Server_ListenAndServe(t *testing.T) {
	tests := []struct {
		name          string
		buildServerFn func(addr string) (Server, error)
		buildClientFn func(host string, port int) (*client.StatsD, error)
	}{
		{
			name:          "udp",
			buildServerFn: NewUDPServer,
			buildClientFn: func(host string, port int) (*client.StatsD, error) {
				return client.NewStatsD(client.UDP, host, port)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := testutil.GetAvailableLocalNetworkAddress(t, "udp")

			// Endpoint should be free.
			ln0, err := net.ListenPacket("udp", addr)
			require.NoError(t, err)
			require.NotNil(t, ln0)

			// Ensure that the endpoint wasn't something like ":0" by checking that a second listener will fail.
			ln1, err := net.ListenPacket("udp", addr)
			require.Error(t, err)
			require.Nil(t, ln1)

			// Unbind the local address so the mock UDP service can use it
			ln0.Close()

			srv, err := tt.buildServerFn(addr)
			require.NoError(t, err)
			require.NotNil(t, srv)

			host, portStr, err := net.SplitHostPort(addr)
			require.NoError(t, err)
			port, err := strconv.Atoi(portStr)
			require.NoError(t, err)

			mc := new(consumertest.MetricsSink)
			p := &protocol.StatsDParser{}
			require.NoError(t, err)
			mr := NewMockReporter(1)
			transferChan := make(chan Metric, 10)

			wgListenAndServe := sync.WaitGroup{}
			wgListenAndServe.Add(1)
			go func() {
				defer wgListenAndServe.Done()
				assert.Error(t, srv.ListenAndServe(p, mc, mr, transferChan))
			}()

			runtime.Gosched()

			gc, err := tt.buildClientFn(host, port)
			require.NoError(t, err)
			require.NotNil(t, gc)
			err = gc.SendMetric(client.Metric{
				Name:  "test.metric",
				Value: "42",
				Type:  "c",
			})
			assert.NoError(t, err)
			runtime.Gosched()
			err = gc.Disconnect()
			assert.NoError(t, err)

			// Keep trying until we're timed out or got a result
			assert.Eventually(t, func() bool {
				return len(transferChan) > 0
			}, 10*time.Second, 500*time.Millisecond)

			// Close the server connection, this will cause ListenAndServer to error out and the deferred wgListenAndServe.Done will fire
			err = srv.Close()
			assert.NoError(t, err)

			wgListenAndServe.Wait()
			assert.Equal(t, 1, len(transferChan))
		})
	}
}
