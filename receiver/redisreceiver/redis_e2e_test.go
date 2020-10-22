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

// +build integration

package redisreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"

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

func TestIntegration(t *testing.T) {
	d := container.New(t)
	c := d.StartImage("docker.io/library/redis:6.0.3", container.WithPortReady(6379))

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*config)
	cfg.Endpoint = c.AddrForPort(6379)

	consumer := new(consumertest.MetricsSink)
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	rcvr, err := f.CreateMetricsReceiver(context.Background(), params, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 15*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, rcvr.Shutdown(context.Background()))
}
