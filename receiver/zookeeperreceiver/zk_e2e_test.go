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

//go:build integration
// +build integration

package zookeeperreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type testHost struct {
	component.Host
	t *testing.T
}

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

const zkPort = 2181

func TestIntegration(t *testing.T) {
	tests := []struct {
		name               string
		image              string
		expectedNumMetrics int
		env                map[string]string
	}{
		{
			name:               "3.4.14",
			image:              "docker.io/library/zookeeper:3.4",
			expectedNumMetrics: 14,
			env: map[string]string{
				"ZOO_4LW_COMMANDS_WHITELIST": "srvr,mntr",
				"ZOO_STANDALONE_ENABLED":     "false",
			},
		},
		{
			name:               "3.5.5-standalone",
			image:              "docker.io/library/zookeeper:3.5.5",
			expectedNumMetrics: 13,
			env: map[string]string{
				"ZOO_4LW_COMMANDS_WHITELIST": "srvr,mntr",
			},
		},
		{
			name:               "3.5.5",
			image:              "docker.io/library/zookeeper:3.5.5",
			expectedNumMetrics: 16,
			env: map[string]string{
				"ZOO_4LW_COMMANDS_WHITELIST": "srvr,mntr",
				"ZOO_STANDALONE_ENABLED":     "false",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			req := testcontainers.ContainerRequest{
				Image:        test.image,
				ExposedPorts: []string{fmt.Sprintf("%d/tcp", zkPort)},
				Env:          test.env,
				WaitingFor:   wait.ForListeningPort("2181/tcp"),
			}
			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			})
			require.Nil(t, err)

			mappedPort, err := container.MappedPort(ctx, "2181")
			require.Nil(t, err)

			hostIP, err := container.Host(ctx)
			require.Nil(t, err)

			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.Endpoint = fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

			consumer := new(consumertest.MetricsSink)

			rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumer)
			require.NoError(t, err, "failed creating metrics receiver")
			require.NoError(t, rcvr.Start(context.Background(), &testHost{t: t}))

			t.Log("waiting for metrics...")
			require.Eventuallyf(t,
				func() bool {
					return consumer.DataPointCount() >= test.expectedNumMetrics
				},
				20*time.Second, 2*time.Second,
				fmt.Sprintf(
					"received %d metrics, expecting %d",
					consumer.DataPointCount(),
					test.expectedNumMetrics,
				),
			)
			t.Log("metrics received...")

			require.NoError(t, rcvr.Shutdown(context.Background()))
		})
	}
}
