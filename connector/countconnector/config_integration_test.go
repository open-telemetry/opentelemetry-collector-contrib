// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package countconnector

import (
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

// Ensure basic count connector configs produce expected metrics.
// See ./testdata/config-*.yaml for specifics.
// Test cases:
// 1) `count:` with empty subconfig uses defaults.
// 2) `count::logs:` with empty subconfig uses defaults.
// 3) `count::logs::<custom metric>` overrides default.

func newCountConfigFromYAML(t *testing.T, yamlFile string) *Config {
	t.Helper()

	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)
	factories.Connectors[metadata.Type] = NewFactory()

	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", yamlFile), factories)
	require.NoError(t, err)

	ccfg, ok := cfg.Connectors[component.NewID(metadata.Type)].(*Config)
	require.True(t, ok)
	require.NotNil(t, ccfg)

	require.NoError(t, ccfg.Validate())
	return ccfg
}

func runLogsToMetrics(t *testing.T, ccfg *Config, logs plog.Logs) pmetric.Metrics {
	t.Helper()

	sink := new(consumertest.MetricsSink)
	conn, err := NewFactory().CreateLogsToMetrics(t.Context(), connectortest.NewNopSettings(metadata.Type), ccfg, sink)
	require.NoError(t, err)
	require.NoError(t, conn.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, conn.Shutdown(t.Context())) })

	require.NoError(t, conn.ConsumeLogs(t.Context(), logs))
	all := sink.AllMetrics()
	require.Len(t, all, 1)
	return all[0]
}

// 1) connectors: count: (empty)
func TestCount_EmptyTopLevel_ProducesDefaultLogRecordCount(t *testing.T) {
	ccfg := newCountConfigFromYAML(t, "config-count-empty.yaml")

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
}

// 2) connectors: count: logs: (empty)
func TestCount_EmptyLogsSection_ProducesDefaultLogRecordCount(t *testing.T) {
	ccfg := newCountConfigFromYAML(t, "config-count-logs-empty.yaml")

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
}

// 3) connectors: count: logs: with a specific metric and condition
func TestCount_SpecificLogMetricWithCondition_Works(t *testing.T) {
	ccfg := newCountConfigFromYAML(t, "config-count-logs-specific.yaml")

	// Build minimal input logs: 2 matching, 1 non-matching
	ld := plog.NewLogs()
	rls := ld.ResourceLogs().AppendEmpty()
	sls := rls.ScopeLogs().AppendEmpty()
	{
		lr := sls.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("log.file.name", "test.in.log")
	}
	{
		lr := sls.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("log.file.name", "test.in.log")
	}
	{
		lr := sls.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("log.file.name", "other.log")
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
				if m.Name() != "log.file.name" {
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
	require.True(t, found, "expected metric 'log.file.name' not found")
}
