// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	_, err := createMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	assert.NoError(t, err)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	_, err := createTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	assert.NoError(t, err)
}

func TestCreateTracesExporterNoAccessToken(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.Realm = "us0"

	_, err := createTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	assert.EqualError(t, err, "access_token is required")
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	exp, err := factory.CreateMetricsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	expCfg := cfg.(*Config)
	expCfg.AccessToken = "testToken"
	expCfg.Realm = "us1"
	exp, err = factory.CreateMetricsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	logExp, err := factory.CreateLogsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, logExp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestCreateMetricsExporter_CustomConfig(t *testing.T) {
	config := &Config{
		AccessToken: "testToken",
		Realm:       "us1",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 2 * time.Second,
			Headers: map[string]configopaque.String{
				"added-entry": "added value",
				"dot.test":    "test",
			},
		},
	}

	te, err := createMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestDefaultTranslationRules(t *testing.T) {
	rules := defaultTranslationRules
	require.NotNil(t, rules, "rules are nil")
	tr, err := translation.NewMetricTranslator(rules, 1)
	require.NoError(t, err)
	data := testMetricsData()

	c, err := translation.NewMetricsConverter(zap.NewNop(), tr, nil, nil, "")
	require.NoError(t, err)
	translated := c.MetricsToSignalFxV2(data)
	require.NotNil(t, translated)

	metrics := make(map[string][]*sfxpb.DataPoint)
	for _, pt := range translated {
		metrics[pt.Metric] = append(metrics[pt.Metric], pt)
	}

	// memory.utilization new metric calculation
	dps, ok := metrics["memory.utilization"]
	require.True(t, ok, "memory.utilization metric not found")
	require.Len(t, dps, 1)
	require.Equal(t, 40.0, *dps[0].Value.DoubleValue)

	// system.disk.operations.total new metric calculation
	dps, ok = metrics["system.disk.operations.total"]
	require.True(t, ok, "system.disk.operations.total metrics not found")
	require.Len(t, dps, 4)
	require.Equal(t, 2, len(dps[0].Dimensions))

	// system.disk.io.total new metric calculation
	dps, ok = metrics["system.disk.io.total"]
	require.True(t, ok, "system.disk.io.total metrics not found")
	require.Len(t, dps, 2)
	require.Equal(t, 2, len(dps[0].Dimensions))
	for _, dp := range dps {
		var directionFound bool
		for _, dim := range dp.Dimensions {
			if dim.Key != "direction" {
				continue
			}
			directionFound = true
			switch dim.Value {
			case "write":
				require.Equal(t, int64(11e9), *dp.Value.IntValue)
			case "read":
				require.Equal(t, int64(3e9), *dp.Value.IntValue)
			}
		}
		require.True(t, directionFound, `missing dimension: direction`)
	}

	// disk_ops.total gauge from system.disk.operations cumulative, where is disk_ops.total
	// is the cumulative across devices and directions.
	dps, ok = metrics["disk_ops.total"]
	require.True(t, ok, "disk_ops.total metrics not found")
	require.Len(t, dps, 1)
	require.Equal(t, int64(8e3), *dps[0].Value.IntValue)
	require.Equal(t, 1, len(dps[0].Dimensions))
	requireDimension(t, dps[0].Dimensions, "host", "host0")

	// system.network.io.total new metric calculation
	dps, ok = metrics["system.network.io.total"]
	require.True(t, ok, "system.network.io.total metrics not found")
	require.Len(t, dps, 2)
	require.Equal(t, 4, len(dps[0].Dimensions))

	// system.network.packets.total new metric calculation
	dps, ok = metrics["system.network.packets.total"]
	require.True(t, ok, "system.network.packets.total metrics not found")
	require.Len(t, dps, 1)
	require.Equal(t, 4, len(dps[0].Dimensions))
	require.Equal(t, int64(350), *dps[0].Value.IntValue)
	requireDimension(t, dps[0].Dimensions, "direction", "receive")

	// network.total new metric calculation
	dps, ok = metrics["network.total"]
	require.True(t, ok, "network.total metrics not found")
	require.Len(t, dps, 1)
	require.Equal(t, 3, len(dps[0].Dimensions))
	require.Equal(t, int64(10e9), *dps[0].Value.IntValue)
}

func requireDimension(t *testing.T, dims []*sfxpb.Dimension, key, val string) {
	var found bool
	for _, dim := range dims {
		if dim.Key != key {
			continue
		}
		found = true
		require.Equal(t, val, dim.Value)
	}
	require.True(t, found, `missing dimension: %s`, key)
}

func testMetricsData() pmetric.Metrics {
	md := pmetric.NewMetrics()

	m1 := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m1.SetName("system.memory.usage")
	m1.SetDescription("Bytes of memory in use")
	m1.SetUnit("bytes")
	dp11 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
	dp11.Attributes().PutStr("state", "used")
	dp11.Attributes().PutStr("host", "host0")
	dp11.Attributes().PutStr("kubernetes_node", "node0")
	dp11.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp11.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp11.SetIntValue(4e9)
	dp12 := m1.Gauge().DataPoints().AppendEmpty()
	dp12.Attributes().PutStr("state", "free")
	dp12.Attributes().PutStr("host", "host0")
	dp12.Attributes().PutStr("kubernetes_node", "node0")
	dp12.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp12.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp12.SetIntValue(6e9)

	sm2 := md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics()
	m2 := sm2.AppendEmpty()
	m2.SetName("system.disk.io")
	m2.SetDescription("Disk I/O.")
	m2.SetEmptySum().SetIsMonotonic(true)
	m2.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp21 := m2.Sum().DataPoints().AppendEmpty()
	dp21.Attributes().PutStr("host", "host0")
	dp21.Attributes().PutStr("direction", "read")
	dp21.Attributes().PutStr("device", "sda1")
	dp21.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp21.SetIntValue(1e9)
	dp22 := m2.Sum().DataPoints().AppendEmpty()
	dp22.Attributes().PutStr("host", "host0")
	dp22.Attributes().PutStr("direction", "read")
	dp22.Attributes().PutStr("device", "sda2")
	dp22.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp22.SetIntValue(2e9)
	dp23 := m2.Sum().DataPoints().AppendEmpty()
	dp23.Attributes().PutStr("host", "host0")
	dp23.Attributes().PutStr("direction", "write")
	dp23.Attributes().PutStr("device", "sda1")
	dp23.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp23.SetIntValue(3e9)
	dp24 := m2.Sum().DataPoints().AppendEmpty()
	dp24.Attributes().PutStr("host", "host0")
	dp24.Attributes().PutStr("direction", "write")
	dp24.Attributes().PutStr("device", "sda2")
	dp24.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp24.SetIntValue(8e9)

	m3 := sm2.AppendEmpty()
	m3.SetName("system.disk.operations")
	m3.SetDescription("Disk operations count.")
	m3.SetUnit("bytes")
	m3.SetEmptySum().SetIsMonotonic(true)
	m3.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp31 := m3.Sum().DataPoints().AppendEmpty()
	dp31.Attributes().PutStr("host", "host0")
	dp31.Attributes().PutStr("direction", "write")
	dp31.Attributes().PutStr("device", "sda1")
	dp31.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp31.SetIntValue(4e3)
	dp32 := m3.Sum().DataPoints().AppendEmpty()
	dp32.Attributes().PutStr("host", "host0")
	dp32.Attributes().PutStr("direction", "read")
	dp32.Attributes().PutStr("device", "sda2")
	dp32.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp32.SetIntValue(6e3)
	dp33 := m3.Sum().DataPoints().AppendEmpty()
	dp33.Attributes().PutStr("host", "host0")
	dp33.Attributes().PutStr("direction", "write")
	dp33.Attributes().PutStr("device", "sda1")
	dp33.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp33.SetIntValue(1e3)
	dp34 := m3.Sum().DataPoints().AppendEmpty()
	dp34.Attributes().PutStr("host", "host0")
	dp34.Attributes().PutStr("direction", "write")
	dp34.Attributes().PutStr("device", "sda2")
	dp34.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp34.SetIntValue(5e3)

	m4 := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m4.SetName("system.disk.operations")
	m4.SetDescription("Disk operations count.")
	m4.SetUnit("bytes")
	m4.SetEmptySum().SetIsMonotonic(true)
	m4.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp41 := m4.Sum().DataPoints().AppendEmpty()
	dp41.Attributes().PutStr("host", "host0")
	dp41.Attributes().PutStr("direction", "read")
	dp41.Attributes().PutStr("device", "sda1")
	dp41.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp41.SetIntValue(6e3)
	dp42 := m4.Sum().DataPoints().AppendEmpty()
	dp42.Attributes().PutStr("host", "host0")
	dp42.Attributes().PutStr("direction", "read")
	dp42.Attributes().PutStr("device", "sda2")
	dp42.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp42.SetIntValue(8e3)
	dp43 := m4.Sum().DataPoints().AppendEmpty()
	dp43.Attributes().PutStr("host", "host0")
	dp43.Attributes().PutStr("direction", "write")
	dp43.Attributes().PutStr("device", "sda1")
	dp43.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp43.SetIntValue(3e3)
	dp44 := m4.Sum().DataPoints().AppendEmpty()
	dp44.Attributes().PutStr("host", "host0")
	dp44.Attributes().PutStr("direction", "write")
	dp44.Attributes().PutStr("device", "sda2")
	dp44.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp44.SetIntValue(7e3)

	sm5 := md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics()
	m5 := sm5.AppendEmpty()
	m5.SetName("system.network.io")
	m5.SetDescription("The number of bytes transmitted and received")
	m5.SetUnit("bytes")
	dp51 := m5.SetEmptyGauge().DataPoints().AppendEmpty()
	dp51.Attributes().PutStr("host", "host0")
	dp51.Attributes().PutStr("direction", "receive")
	dp51.Attributes().PutStr("device", "eth0")
	dp51.Attributes().PutStr("kubernetes_node", "node0")
	dp51.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp51.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp51.SetIntValue(4e9)
	dp52 := m5.Gauge().DataPoints().AppendEmpty()
	dp52.Attributes().PutStr("host", "host0")
	dp52.Attributes().PutStr("direction", "transmit")
	dp52.Attributes().PutStr("device", "eth0")
	dp52.Attributes().PutStr("kubernetes_node", "node0")
	dp52.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp52.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp52.SetIntValue(6e9)

	m6 := sm5.AppendEmpty()
	m6.SetName("system.network.packets")
	m6.SetDescription("The number of packets transferred")
	dp61 := m6.SetEmptyGauge().DataPoints().AppendEmpty()
	dp61.Attributes().PutStr("host", "host0")
	dp61.Attributes().PutStr("direction", "receive")
	dp61.Attributes().PutStr("device", "eth0")
	dp61.Attributes().PutStr("kubernetes_node", "node0")
	dp61.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp61.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp61.SetIntValue(200)
	dp62 := m6.Gauge().DataPoints().AppendEmpty()
	dp62.Attributes().PutStr("host", "host0")
	dp62.Attributes().PutStr("direction", "receive")
	dp62.Attributes().PutStr("device", "eth1")
	dp62.Attributes().PutStr("kubernetes_node", "node0")
	dp62.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp62.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp62.SetIntValue(150)

	sm7 := md.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics()
	m7 := sm7.AppendEmpty()
	m7.SetName("container.memory.working_set")
	m7.SetUnit("bytes")
	dp71 := m7.SetEmptyGauge().DataPoints().AppendEmpty()
	dp71.Attributes().PutStr("host", "host0")
	dp71.Attributes().PutStr("kubernetes_node", "node0")
	dp71.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp71.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp71.SetIntValue(1000)

	m8 := sm7.AppendEmpty()
	m8.SetName("container.memory.page_faults")
	dp81 := m8.SetEmptyGauge().DataPoints().AppendEmpty()
	dp81.Attributes().PutStr("host", "host0")
	dp81.Attributes().PutStr("kubernetes_node", "node0")
	dp81.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp81.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp81.SetIntValue(1000)

	m9 := sm7.AppendEmpty()
	m9.SetName("container.memory.major_page_faults")
	dp91 := m9.SetEmptyGauge().DataPoints().AppendEmpty()
	dp91.Attributes().PutStr("host", "host0")
	dp91.Attributes().PutStr("kubernetes_node", "node0")
	dp91.Attributes().PutStr("kubernetes_cluster", "cluster0")
	dp91.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp91.SetIntValue(1000)

	return md
}

func TestDefaultDiskTranslations(t *testing.T) {
	var pts []*sfxpb.DataPoint
	err := testReadJSON("testdata/json/system.filesystem.usage.json", &pts)
	require.NoError(t, err)

	tr := testGetTranslator(t)
	translated := tr.TranslateDataPoints(zap.NewNop(), pts)
	require.NotNil(t, translated)

	m := map[string][]*sfxpb.DataPoint{}
	for _, pt := range translated {
		l := m[pt.Metric]
		l = append(l, pt)
		m[pt.Metric] = l
	}

	_, ok := m["disk.total"]
	require.False(t, ok)

	_, ok = m["disk.summary_total"]
	require.False(t, ok)

	_, ok = m["df_complex.used_total"]
	require.False(t, ok)

	du, ok := m["disk.utilization"]
	require.True(t, ok)
	require.Equal(t, 4, len(du[0].Dimensions))
	// cheap test for pct conversion
	require.True(t, *du[0].Value.DoubleValue > 1)

	dsu, ok := m["disk.summary_utilization"]
	require.True(t, ok)
	require.Equal(t, 3, len(dsu[0].Dimensions))
	require.True(t, *dsu[0].Value.DoubleValue > 1)
}

func testGetTranslator(t *testing.T) *translation.MetricTranslator {
	rules := defaultTranslationRules
	require.NotNil(t, rules, "rules are nil")
	tr, err := translation.NewMetricTranslator(rules, 3600)
	require.NoError(t, err)
	return tr
}

func TestDefaultCPUTranslations(t *testing.T) {
	var pts1 []*sfxpb.DataPoint
	err := testReadJSON("testdata/json/system.cpu.time.1.json", &pts1)
	require.NoError(t, err)

	var pts2 []*sfxpb.DataPoint
	err = testReadJSON("testdata/json/system.cpu.time.2.json", &pts2)
	require.NoError(t, err)

	tr := testGetTranslator(t)
	log := zap.NewNop()

	// write 'prev' points from which to calculate deltas
	_ = tr.TranslateDataPoints(log, pts1)

	// calculate cpu utilization
	translated2 := tr.TranslateDataPoints(log, pts2)

	m := map[string][]*sfxpb.DataPoint{}
	for _, pt := range translated2 {
		pts := m[pt.Metric]
		pts = append(pts, pt)
		m[pt.Metric] = pts
	}

	cpuUtil := m["cpu.utilization"]
	require.Equal(t, 1, len(cpuUtil))
	for _, pt := range cpuUtil {
		require.Equal(t, 66, int(*pt.Value.DoubleValue))
	}

	cpuUtilPerCore := m["cpu.utilization_per_core"]
	require.Equal(t, 8, len(cpuUtilPerCore))

	cpuNumProcessors := m["cpu.num_processors"]
	require.Equal(t, 1, len(cpuNumProcessors))

	cpuStateMetrics := []string{"cpu.idle", "cpu.interrupt", "cpu.system", "cpu.user"}
	for _, metric := range cpuStateMetrics {
		dps, ok := m[metric]
		require.True(t, ok, fmt.Sprintf("%s metrics not found", metric))
		require.Len(t, dps, 9)
	}
}

func TestHostmetricsCPUTranslations(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	require.NoError(t, setDefaultExcludes(cfg))
	converter, err := translation.NewMetricsConverter(zap.NewNop(), testGetTranslator(t), cfg.ExcludeMetrics, cfg.IncludeMetrics, "")
	require.NoError(t, err)

	md1, err := golden.ReadMetrics(filepath.Join("testdata", "hostmetrics_system_cpu_time_1.yaml"))
	require.NoError(t, err)

	_ = converter.MetricsToSignalFxV2(md1)

	md2, err := golden.ReadMetrics(filepath.Join("testdata", "hostmetrics_system_cpu_time_2.yaml"))
	require.NoError(t, err)

	translated2 := converter.MetricsToSignalFxV2(md2)

	m := map[string][]*sfxpb.DataPoint{}
	for _, pt := range translated2 {
		pts := m[pt.Metric]
		pts = append(pts, pt)
		m[pt.Metric] = pts
	}

	cpuUtil := m["cpu.utilization"]
	require.Len(t, cpuUtil, 1)
	require.Equal(t, sfxpb.MetricType_GAUGE, *cpuUtil[0].MetricType)
	require.Equal(t, 59, int(*cpuUtil[0].Value.DoubleValue))

	cpuNumProcessors := m["cpu.num_processors"]
	require.Len(t, cpuNumProcessors, 1)
	require.Equal(t, sfxpb.MetricType_GAUGE, *cpuNumProcessors[0].MetricType)
	require.Equal(t, 2, int(*cpuNumProcessors[0].Value.IntValue))

	cpuIdle := m["cpu.idle"]
	require.Len(t, cpuIdle, 1)
	require.Equal(t, sfxpb.MetricType_CUMULATIVE_COUNTER, *cpuIdle[0].MetricType)
	require.Equal(t, 590, int(*cpuIdle[0].Value.IntValue))
}

func TestDefaultExcludesTranslated(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	require.NoError(t, setDefaultExcludes(cfg))

	converter, err := translation.NewMetricsConverter(zap.NewNop(), testGetTranslator(t), cfg.ExcludeMetrics, cfg.IncludeMetrics, "")
	require.NoError(t, err)

	var metrics []map[string]string
	err = testReadJSON("testdata/json/non_default_metrics.json", &metrics)
	require.NoError(t, err)

	md := getMetrics(metrics)
	require.Equal(t, 9, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	dps := converter.MetricsToSignalFxV2(md)

	// the default cpu.utilization metric is added after applying the default translations
	// (because cpu.utilization_per_core is supplied) and should not be excluded
	require.Equal(t, 1, len(dps))
	require.Equal(t, "cpu.utilization", dps[0].Metric)

}

func TestDefaultExcludes_not_translated(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	require.NoError(t, setDefaultExcludes(cfg))

	converter, err := translation.NewMetricsConverter(zap.NewNop(), nil, cfg.ExcludeMetrics, cfg.IncludeMetrics, "")
	require.NoError(t, err)

	var metrics []map[string]string
	err = testReadJSON("testdata/json/non_default_metrics_otel_convention.json", &metrics)
	require.NoError(t, err)

	md := getMetrics(metrics)
	require.Equal(t, 69, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	dps := converter.MetricsToSignalFxV2(md)
	require.Equal(t, 0, len(dps))
}

// Benchmark test for default translation rules on an example hostmetrics dataset.
func BenchmarkMetricConversion(b *testing.B) {
	rules := defaultTranslationRules
	require.NotNil(b, rules, "rules are nil")
	tr, err := translation.NewMetricTranslator(rules, 1)
	require.NoError(b, err)

	c, err := translation.NewMetricsConverter(zap.NewNop(), tr, nil, nil, "")
	require.NoError(b, err)

	bytes, err := os.ReadFile("testdata/json/hostmetrics.json")
	require.NoError(b, err)

	unmarshaller := &pmetric.JSONUnmarshaler{}
	metrics, err := unmarshaller.UnmarshalMetrics(bytes)
	require.NoError(b, err)

	for n := 0; n < b.N; n++ {
		translated := c.MetricsToSignalFxV2(metrics)
		require.NotNil(b, translated)
	}
}

func getMetrics(metrics []map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ilms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilms.Metrics().EnsureCapacity(len(metrics))

	for _, mp := range metrics {
		m := ilms.Metrics().AppendEmpty()
		// Set data type to some arbitrary since it does not matter for this test.
		dp := m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(0)
		attributesMap := dp.Attributes()
		for k, v := range mp {
			if v == "" {
				m.SetName(k)
				continue
			}
			attributesMap.PutStr(k, v)
		}
	}
	return md
}

func testReadJSON(f string, v interface{}) error {
	bytes, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &v)
}
