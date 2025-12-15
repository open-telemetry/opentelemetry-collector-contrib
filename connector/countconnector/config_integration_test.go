// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package countconnector

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func getValidatedCountConfigFromYAML(t *testing.T, configPath string) (*Config, error) {
	t.Helper()

	factories, err := otelcoltest.NopFactories()
	if err != nil {
		return nil, fmt.Errorf("create nop factories: %w", err)
	}
	factories.Connectors[metadata.Type] = NewFactory()

	cfg, err := otelcoltest.LoadConfigAndValidate(configPath, factories)
	if err != nil {
		return nil, fmt.Errorf("load and validate config %q: %w", configPath, err)
	}

	connectorID := component.NewID(metadata.Type)
	ccfg, ok := cfg.Connectors[connectorID].(*Config)
	if !ok {
		return nil, fmt.Errorf("connector %q config is not of type *Config", connectorID)
	}
	if ccfg == nil {
		return nil, fmt.Errorf("connector %q config is nil", connectorID)
	}

	return ccfg, nil
}

func runLogsToMetrics(t *testing.T, ccfg *Config, logs plog.Logs) pmetric.Metrics {
	t.Helper()

	sink := new(consumertest.MetricsSink)
	conn, err := NewFactory().CreateLogsToMetrics(t.Context(), connectortest.NewNopSettings(metadata.Type), ccfg, sink)
	require.NoError(t, err)
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, conn.Shutdown(context.Background())) }) //nolint:usetesting

	require.NoError(t, conn.ConsumeLogs(t.Context(), logs))
	all := sink.AllMetrics()
	require.Len(t, all, 1)
	return all[0]
}

// Test that empty subconfigs use defaults.
func TestCount_EmptySubconfig_ProducesDefaultLogRecordCount(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
	}{
		{
			name:       "empty top-level count config",
			configFile: "config-count-empty.yaml",
		},
		{
			name:       "empty logs section",
			configFile: "config-count-logs-empty.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ccfg, err := getValidatedCountConfigFromYAML(t, filepath.Join("testdata", tt.configFile))
			require.NoError(t, err)
			require.NotNil(t, ccfg)

			in, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.yaml"))
			require.NoError(t, err)

			got := runLogsToMetrics(t, ccfg, in)
			want, err := golden.ReadMetrics(filepath.Join("testdata", "logs", "zero_conditions.yaml"))
			require.NoError(t, err)

			require.NoError(t, pmetrictest.CompareMetrics(
				want, got,
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
			))
		})
	}
}

// Test that `count::logs::<custom metric>` overrides default.
func TestCount_SpecificLogMetricWithCondition_Works(t *testing.T) {
	ccfg, err := getValidatedCountConfigFromYAML(t, filepath.Join("testdata", "config-count-logs-specific.yaml"))
	require.NoError(t, err)
	require.NotNil(t, ccfg)

	metricName := "log.file.name"

	// Build minimal input logs: 2 matching, 1 non-matching
	ld := plog.NewLogs()
	rls := ld.ResourceLogs().AppendEmpty()
	sls := rls.ScopeLogs().AppendEmpty()
	{
		lr := sls.LogRecords().AppendEmpty()
		lr.Attributes().PutStr(metricName, "test.in.log")
	}
	{
		lr := sls.LogRecords().AppendEmpty()
		lr.Attributes().PutStr(metricName, "test.in.log")
	}
	{
		lr := sls.LogRecords().AppendEmpty()
		lr.Attributes().PutStr(metricName, "other.log")
	}

	got := runLogsToMetrics(t, ccfg, ld)

	// Assert we emitted a single metric named "log.file.name" with total value 2
	found := false
	rmSlice := got.ResourceMetrics()
	for i := 0; i < rmSlice.Len(); i++ {
		smSlice := rmSlice.At(i).ScopeMetrics()
		for j := 0; j < smSlice.Len(); j++ {
			ms := smSlice.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != metricName {
					continue
				}
				found = true
				var total int64
				dps := m.Sum().DataPoints()
				for d := 0; d < dps.Len(); d++ {
					total += dps.At(d).IntValue()
				}
				assert.Equal(t, int64(2), total)
			}
		}
	}
	require.True(t, found, "expected metric %q not found", metricName)
}
