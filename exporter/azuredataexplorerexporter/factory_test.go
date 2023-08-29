// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter/internal/metadata"
)

// Given a new factory and no-op exporter , the NewMetric exporter should work.
// We could add additional failing tests if the config is wrong (using Validate) , but that is already done on config
func TestCreateMetricsExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NotNil(t, exporter)
	assert.NoError(t, err)

	// Test the errors, as the auth will fail here.Error while getting token, as the cluster is empty
	testMetrics := pmetric.NewMetrics()
	m := testMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("MetricsUnitTest")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(42.42)
	err = exporter.ConsumeMetrics(context.Background(), testMetrics)
	assert.Error(t, err)
	assert.Nil(t, exporter.Shutdown(context.Background()))
}

// Given a new factory and no-op exporter , the NewMetric exporter should work.
// We could add additional failing tests if the config is wrong (using Validate) , but that is already done on config
func TestCreateMetricsExporterWhenIngestEmpty(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	// Load the #3 which has empty. This
	assert.Panics(t, func() { _, _ = factory.CreateMetricsExporter(context.Background(), params, cfg) })
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, otelDb, cfg.Database)
	assert.Equal(t, queuedIngestTest, cfg.IngestionType)
}

// Given a new factory and no-op exporter , the LogExporter exporter should work.
// We could add additional failing tests if the config is wrong (using Validate) , but that is already done on config
func TestCreateLogsExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	// Load the #3 which has empty. This
	assert.NotNil(t, exporter)
	assert.NoError(t, err)

	// Test the errors, as the auth will fail here.Error while getting token, as the cluster is empty
	testLogs := plog.NewLogs()
	testLogs.ResourceLogs().AppendEmpty()
	testLogs.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
	testLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	testLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetTraceID([16]byte{1, 2, 3, 4})
	// This will fail with auth failure
	err = exporter.ConsumeLogs(context.Background(), testLogs)
	assert.Error(t, err)
	assert.Nil(t, exporter.Shutdown(context.Background()))
}

// Given a new factory and no-op exporter , the NewLogs exporter should work.
// We could add additional failing tests if the config is wrong (using Validate) , but that is already done on config
func TestCreateLogsExporterWhenIngestEmpty(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	// Load the #3 which has empty
	// exporter, err := factory.CreateLogsExporter(context.Background(), params, cfg)
	assert.Panics(t, func() { _, _ = factory.CreateLogsExporter(context.Background(), params, cfg) })
}

// Given a new factory and no-op exporter , the LogExporter exporter should work.
// We could add additional failing tests if the config is wrong (using Validate) , but that is already done on config
func TestCreateTracesExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	exporter, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NotNil(t, exporter)
	assert.NoError(t, err)

	// Error while getting token, as the cluster is empty
	testTraces := ptrace.NewTraces()
	rs := testTraces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty()
	err = exporter.ConsumeTraces(context.Background(), testTraces)
	assert.Error(t, err)
	assert.Nil(t, exporter.Shutdown(context.Background()))
}

// Given a new factory and no-op exporter , the NewLogs exporter should work.
// We could add additional failing tests if the config is wrong (using Validate) , but that is already done on config
func TestCreateTracesExporterWhenIngestEmpty(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	// Load the #3 which has empty
	assert.Panics(t, func() { _, _ = factory.CreateTracesExporter(context.Background(), params, cfg) })
}
