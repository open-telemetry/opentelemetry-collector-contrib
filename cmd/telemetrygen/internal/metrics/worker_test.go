// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/global"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

func TestFixedNumberOfMetrics(t *testing.T) {
	// prepare
	manualReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(manualReader))
	global.SetMeterProvider(metricProvider)

	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumMetrics: 1,
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, logger))

	time.Sleep(1 * time.Second)

	// verify
	m, err := manualReader.Collect(context.Background())
	require.Len(t, m.ScopeMetrics, 1)
	require.NoError(t, err)
	assert.Len(t, m.ScopeMetrics[0].Metrics, 1)
}

func TestRateOfMetrics(t *testing.T) {
	// prepare
	manualReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(manualReader))
	global.SetMeterProvider(metricProvider)

	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
	}

	// sanity check
	m, err := manualReader.Collect(context.Background())
	require.Len(t, m.ScopeMetrics, 0)
	require.NoError(t, err)

	// test
	require.NoError(t, Run(cfg, zap.NewNop()))

	// verify
	// the minimum acceptable number of metrics for the rate of 10/sec for half a second
	m, err = manualReader.Collect(context.Background())
	require.Len(t, m.ScopeMetrics, 1)
	require.NoError(t, err)
	assert.True(t, len(m.ScopeMetrics[0].Metrics) >= 6, "there should have been more than 6 metrics, had %d", len(m.ScopeMetrics[0].Metrics))
	// the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(m.ScopeMetrics[0].Metrics) <= 20, "there should have been less than 20 metrics, had %d", len(m.ScopeMetrics[0].Metrics))
}

func TestUnthrottled(t *testing.T) {
	// prepare
	manualReader := sdkmetric.NewManualReader()
	metricProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(manualReader))
	global.SetMeterProvider(metricProvider)

	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
	}

	// sanity check
	m, err := manualReader.Collect(context.Background())
	require.Len(t, m.ScopeMetrics, 0)
	require.NoError(t, err)

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, logger))

	collected, err := manualReader.Collect(context.Background())
	require.Len(t, collected.ScopeMetrics, 1)
	count := 0
	for _, m := range collected.ScopeMetrics[0].Metrics {
		sum := m.Data.(metricdata.Sum[int64])
		count += len(sum.DataPoints)
	}
	assert.True(t, count > 100, "there should have been more than 100 metrics, had %d", count)
	require.NoError(t, err)
}
