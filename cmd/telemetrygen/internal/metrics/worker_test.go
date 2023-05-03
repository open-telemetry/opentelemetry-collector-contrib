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
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

type mockExporter struct {
	rms []*metricdata.ResourceMetrics
}

func (m *mockExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.DeltaTemporality
}

func (m *mockExporter) Aggregation(kind sdkmetric.InstrumentKind) aggregation.Aggregation {
	return aggregation.Default{}
}

func (m *mockExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	m.rms = append(m.rms, metrics)
	return nil
}

func (m *mockExporter) ForceFlush(ctx context.Context) error {
	return nil
}

func (m *mockExporter) Shutdown(ctx context.Context) error {
	return nil
}

func TestFixedNumberOfMetrics(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumMetrics: 5,
	}

	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, exp.rms, 5)
}

func TestRateOfMetrics(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
	}
	exp := &mockExporter{}

	// test
	require.NoError(t, Run(cfg, exp, zap.NewNop()))

	// verify
	// the minimum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(exp.rms) >= 6, "there should have been more than 6 metrics, had %d", len(exp.rms))
	// the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(exp.rms) <= 20, "there should have been less than 20 metrics, had %d", len(exp.rms))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
	}
	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	assert.True(t, len(exp.rms) > 100, "there should have been more than 100 metrics, had %d", len(exp.rms))
}
