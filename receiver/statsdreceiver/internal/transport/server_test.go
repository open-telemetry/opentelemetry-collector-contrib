// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"io"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport/client"
)

func Test_Server_ListenAndServe(t *testing.T) {
	tests := []struct {
		name              string
		transport         Transport
		buildServerFn     func(transport Transport, addr string) (Server, error)
		getFreeEndpointFn func(tb testing.TB, transport string) string
		buildClientFn     func(transport string, address string) (*client.StatsD, error)
	}{
		{
			name:              "udp",
			transport:         UDP,
			getFreeEndpointFn: testutil.GetAvailableLocalNetworkAddress,
			buildServerFn:     NewUDPServer,
			buildClientFn:     client.NewStatsD,
		},
		{
			name:              "tcp",
			transport:         TCP,
			getFreeEndpointFn: testutil.GetAvailableLocalNetworkAddress,
			buildServerFn:     NewTCPServer,
			buildClientFn:     client.NewStatsD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := tt.getFreeEndpointFn(t, tt.name)
			testFreeEndpoint(t, tt.name, addr)

			srv, err := tt.buildServerFn(tt.transport, addr)
			require.NoError(t, err)
			require.NotNil(t, srv)

			mc := new(consumertest.MetricsSink)
			require.NoError(t, err)
			mr := NewMockReporter(1)
			transferChan := make(chan Metric, 10)

			wgListenAndServe := sync.WaitGroup{}
			wgListenAndServe.Add(1)
			go func() {
				defer wgListenAndServe.Done()
				assert.Error(t, srv.ListenAndServe(mc, mr, transferChan))
			}()

			runtime.Gosched()

			gc, err := tt.buildClientFn(tt.transport.String(), addr)
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
			assert.Len(t, transferChan, 1)
		})
	}
}

func testFreeEndpoint(t *testing.T, transport string, address string) {
	t.Helper()

	var ln0, ln1 io.Closer
	var err0, err1 error

	trans := NewTransport(transport)
	require.NotEqual(t, trans, Transport(""))

	if trans.IsPacketTransport() {
		// Endpoint should be free.
		ln0, err0 = net.ListenPacket(transport, address)
		ln1, err1 = net.ListenPacket(transport, address)
	}

	if trans.IsStreamTransport() {
		// Endpoint should be free.
		ln0, err0 = net.Listen(transport, address)
		ln1, err1 = net.Listen(transport, address)
	}

	// Endpoint should be free.
	require.NoError(t, err0)
	require.NotNil(t, ln0)

	// Ensure that the endpoint wasn't something like ":0" by checking that a second listener will fail.
	require.Error(t, err1)
	require.Nil(t, ln1)

	// Unbind the local address so the mock UDP service can use it
	require.NoError(t, ln0.Close())
}
