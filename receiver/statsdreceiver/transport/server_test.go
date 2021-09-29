// Copyright 2020, OpenTelemetry Authors
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

package transport

import (
	"net"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport/client"
)

func Test_Server_ListenAndServe(t *testing.T) {
	t.Skip("Test is unstable, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1426")

	tests := []struct {
		name          string
		buildServerFn func(addr string) (Server, error)
		buildClientFn func(host string, port int) (*client.StatsD, error)
	}{
		{
			name: "udp",
			buildServerFn: func(addr string) (Server, error) {
				return NewUDPServer(addr)
			},
			buildClientFn: func(host string, port int) (*client.StatsD, error) {
				return client.NewStatsD(client.UDP, host, port)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := testutil.GetAvailableLocalAddress(t)
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
			var transferChan = make(chan string, 10)

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

			err = srv.Close()
			assert.NoError(t, err)

			wgListenAndServe.Wait()
			assert.Equal(t, 1, len(transferChan))
		})
	}
}
