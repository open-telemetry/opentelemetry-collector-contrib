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

package memcachedreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/memcached:1.6-alpine",
		ExposedPorts: []string{"11211/tcp"},
		WaitingFor:   wait.ForListeningPort("11211/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)

	mappedPort, err := container.MappedPort(ctx, "11211")
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	consumer := new(consumertest.MetricsSink)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	require.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 15*time.Second, 1*time.Second, "failed to receive at least 5 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]

	expectedMetrics, err := golden.ReadMetrics("./testdata/expected_metrics/test_scraper/expected.yaml")
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}
