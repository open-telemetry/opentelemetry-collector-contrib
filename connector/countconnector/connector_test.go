// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package countconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestTracesToMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sink := &consumertest.MetricsSink{}
	conn, err := factory.CreateTracesToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	testTraces0, err := golden.ReadTraces(filepath.Join("testdata", "traces_to_metrics", "traces_0.json"))
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeTraces(context.Background(), testTraces0))

	testTraces1, err := golden.ReadTraces(filepath.Join("testdata", "traces_to_metrics", "traces_1.json"))
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeTraces(context.Background(), testTraces1))

	allMetrics := sink.AllMetrics()
	assert.Equal(t, 2, len(allMetrics))

	expectedMetrics0, err := golden.ReadMetrics(filepath.Join("testdata", "traces_to_metrics", "expected_0.json"))
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics0, allMetrics[0], pmetrictest.IgnoreTimestamp()))

	expectedMetrics1, err := golden.ReadMetrics(filepath.Join("testdata", "traces_to_metrics", "expected_1.json"))
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1, allMetrics[1], pmetrictest.IgnoreTimestamp()))
}

func TestMetricsToMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sink := &consumertest.MetricsSink{}
	conn, err := factory.CreateMetricsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	testMetrics0, err := golden.ReadMetrics(filepath.Join("testdata", "metrics_to_metrics", "metrics_0.json"))
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeMetrics(context.Background(), testMetrics0))

	testMetrics1, err := golden.ReadMetrics(filepath.Join("testdata", "metrics_to_metrics", "metrics_1.json"))
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeMetrics(context.Background(), testMetrics1))

	allMetrics := sink.AllMetrics()
	assert.Equal(t, 2, len(allMetrics))

	expectedMetrics0, err := golden.ReadMetrics(filepath.Join("testdata", "metrics_to_metrics", "expected_0.json"))
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics0, allMetrics[0], pmetrictest.IgnoreTimestamp()))

	expectedMetrics1, err := golden.ReadMetrics(filepath.Join("testdata", "metrics_to_metrics", "expected_1.json"))
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1, allMetrics[1], pmetrictest.IgnoreTimestamp()))
}

func TestLogsToMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sink := &consumertest.MetricsSink{}
	conn, err := factory.CreateLogsToMetrics(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.False(t, conn.Capabilities().MutatesData)

	require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, conn.Shutdown(context.Background()))
	}()

	testLogs0, err := golden.ReadLogs(filepath.Join("testdata", "logs_to_metrics", "logs_0.json"))
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs0))

	testLogs1, err := golden.ReadLogs(filepath.Join("testdata", "logs_to_metrics", "logs_1.json"))
	assert.NoError(t, err)
	assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs1))

	allMetrics := sink.AllMetrics()
	assert.Equal(t, 2, len(allMetrics))

	expectedMetrics0, err := golden.ReadMetrics(filepath.Join("testdata", "logs_to_metrics", "expected_0.json"))
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics0, allMetrics[0], pmetrictest.IgnoreTimestamp()))

	expectedMetrics1, err := golden.ReadMetrics(filepath.Join("testdata", "logs_to_metrics", "expected_1.json"))
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1, allMetrics[1], pmetrictest.IgnoreTimestamp()))
}
