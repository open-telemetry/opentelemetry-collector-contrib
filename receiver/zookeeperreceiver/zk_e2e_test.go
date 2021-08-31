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

//go:build integration
// +build integration

package zookeeperreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/container"
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
		env                []string
	}{
		{
			name:               "3.4.14",
			image:              "docker.io/library/zookeeper:3.4",
			expectedNumMetrics: 14,
			env:                []string{"ZOO_4LW_COMMANDS_WHITELIST=srvr,mntr", "ZOO_STANDALONE_ENABLED=false"},
		},
		{
			name:               "3.5.5-standalone",
			image:              "docker.io/library/zookeeper:3.5.5",
			expectedNumMetrics: 13,
			env:                []string{"ZOO_4LW_COMMANDS_WHITELIST=srvr,mntr"},
		},
		{
			name:               "3.5.5",
			image:              "docker.io/library/zookeeper:3.5.5",
			expectedNumMetrics: 16,
			env:                []string{"ZOO_4LW_COMMANDS_WHITELIST=srvr,mntr", "ZOO_STANDALONE_ENABLED=false"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := container.New(t)
			c := d.StartImageWithEnv(test.image, test.env, container.WithPortReady(zkPort))

			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.Endpoint = c.AddrForPort(zkPort)

			consumer := new(consumertest.MetricsSink)

			rcvr, err := f.CreateMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, consumer)
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
