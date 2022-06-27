// Copyright 2019, OpenTelemetry Authors
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

// nolint:errcheck
package signalfxexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	_, err := createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	assert.NoError(t, err)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	_, err := createTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	assert.NoError(t, err)
}

func TestCreateTracesExporterNoAccessToken(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.Realm = "us0"

	_, err := createTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
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
		componenttest.NewNopExporterCreateSettings(),
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	expCfg := cfg.(*Config)
	expCfg.AccessToken = "testToken"
	expCfg.Realm = "us1"
	exp, err = factory.CreateMetricsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	logExp, err := factory.CreateLogsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, logExp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestCreateMetricsExporter_CustomConfig(t *testing.T) {
	config := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		AccessToken:      "testToken",
		Realm:            "us1",
		Headers: map[string]string{
			"added-entry": "added value",
			"dot.test":    "test",
		},
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 2 * time.Second},
	}

	te, err := createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestFactory_CreateMetricsExporterFails(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		errorMessage string
	}{
		{
			name: "negative_duration",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				AccessToken:      "testToken",
				Realm:            "lab",
				TimeoutSettings:  exporterhelper.TimeoutSettings{Timeout: -2 * time.Second},
			},
			errorMessage: "failed to process \"signalfx\" config: cannot have a negative \"timeout\"",
		},
		{
			name: "empty_realm_and_urls",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				AccessToken:      "testToken",
			},
			errorMessage: "failed to process \"signalfx\" config: requires a non-empty \"realm\"," +
				" or \"ingest_url\" and \"api_url\" should be explicitly set",
		},
		{
			name: "empty_realm_and_api_url",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				AccessToken:      "testToken",
				IngestURL:        "http://localhost:123",
			},
			errorMessage: "failed to process \"signalfx\" config: requires a non-empty \"realm\"," +
				" or \"ingest_url\" and \"api_url\" should be explicitly set",
		},
		{
			name: "negative_MaxConnections",
			config: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				AccessToken:      "testToken",
				Realm:            "lab",
				IngestURL:        "http://localhost:123",
				APIURL:           "https://api.us1.signalfx.com/",
				MaxConnections:   -10,
			},
			errorMessage: "failed to process \"signalfx\" config: cannot have a negative \"max_connections\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te, err := createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), tt.config)
			assert.EqualError(t, err, tt.errorMessage)
			assert.Nil(t, te)
		})
	}
}

func TestDefaultTranslationRules(t *testing.T) {
	rules, err := loadDefaultTranslationRules()
	require.NoError(t, err)
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

	// system.network.operations.total new metric calculation
	dps, ok = metrics["system.disk.operations.total"]
	require.True(t, ok, "system.network.operations.total metrics not found")
	require.Len(t, dps, 4)
	require.Equal(t, 2, len(dps[0].Dimensions))

	// system.network.io.total new metric calculation
	dps, ok = metrics["system.disk.io.total"]
	require.True(t, ok, "system.network.io.total metrics not found")
	require.Len(t, dps, 2)
	require.Equal(t, 2, len(dps[0].Dimensions))
	for _, dp := range dps {
		require.Equal(t, "direction", dp.Dimensions[0].Key)
		switch dp.Dimensions[1].Value {
		case "write":
			require.Equal(t, int64(11e9), *dp.Value.IntValue)
		case "read":
			require.Equal(t, int64(3e9), *dp.Value.IntValue)
		}
	}

	// disk_ops.total gauge from system.disk.operations cumulative, where is disk_ops.total
	// is the cumulative across devices and directions.
	dps, ok = metrics["disk_ops.total"]
	require.True(t, ok, "disk_ops.total metrics not found")
	require.Len(t, dps, 1)
	require.Equal(t, int64(8e3), *dps[0].Value.IntValue)
	require.Equal(t, 1, len(dps[0].Dimensions))
	require.Equal(t, "host", dps[0].Dimensions[0].Key)
	require.Equal(t, "host0", dps[0].Dimensions[0].Value)

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
	require.Equal(t, "direction", dps[0].Dimensions[0].Key)
	require.Equal(t, "receive", dps[0].Dimensions[0].Value)

	// network.total new metric calculation
	dps, ok = metrics["network.total"]
	require.True(t, ok, "network.total metrics not found")
	require.Len(t, dps, 1)
	require.Equal(t, 3, len(dps[0].Dimensions))
	require.Equal(t, int64(10e9), *dps[0].Value.IntValue)
}

func TestCreateMetricsExporterWithDefaultExcludeMetrics(t *testing.T) {
	config := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		AccessToken:      "testToken",
		Realm:            "us1",
	}

	te, err := createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), config)
	require.NoError(t, err)
	require.NotNil(t, te)

	// Validate that default excludes are always loaded.
	assert.Equal(t, 12, len(config.ExcludeMetrics))
}

func TestCreateMetricsExporterWithExcludeMetrics(t *testing.T) {
	config := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		AccessToken:      "testToken",
		Realm:            "us1",
		ExcludeMetrics: []dpfilters.MetricFilter{
			{
				MetricNames: []string{"metric1"},
			},
		},
	}

	te, err := createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), config)
	require.NoError(t, err)
	require.NotNil(t, te)

	// Validate that default excludes are always loaded.
	assert.Equal(t, 13, len(config.ExcludeMetrics))
}

func TestCreateMetricsExporterWithEmptyExcludeMetrics(t *testing.T) {
	config := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		AccessToken:      "testToken",
		Realm:            "us1",
		ExcludeMetrics:   []dpfilters.MetricFilter{},
	}

	te, err := createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), config)
	require.NoError(t, err)
	require.NotNil(t, te)

	// Validate that default excludes are overridden when exclude metrics
	// is explicitly set to an empty slice.
	assert.Equal(t, 0, len(config.ExcludeMetrics))
}

func testMetricsData() pmetric.Metrics {
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	m1 := ms.AppendEmpty()
	m1.SetName("system.memory.usage")
	m1.SetDescription("Bytes of memory in use")
	m1.SetUnit("bytes")
	m1.SetDataType(pmetric.MetricDataTypeGauge)
	dp11 := m1.Gauge().DataPoints().AppendEmpty()
	dp11.Attributes().InsertString("state", "used")
	dp11.Attributes().InsertString("host", "host0")
	dp11.Attributes().InsertString("kubernetes_node", "node0")
	dp11.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp11.Attributes().Sort()
	dp11.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp11.SetIntVal(4e9)
	dp12 := m1.Gauge().DataPoints().AppendEmpty()
	dp12.Attributes().InsertString("state", "free")
	dp12.Attributes().InsertString("host", "host0")
	dp12.Attributes().InsertString("kubernetes_node", "node0")
	dp12.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp12.Attributes().Sort()
	dp12.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp12.SetIntVal(6e9)

	m2 := ms.AppendEmpty()
	m2.SetName("system.disk.io")
	m2.SetDescription("Disk I/O.")
	m2.SetDataType(pmetric.MetricDataTypeSum)
	m2.Sum().SetIsMonotonic(true)
	m2.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	dp21 := m2.Sum().DataPoints().AppendEmpty()
	dp21.Attributes().InsertString("host", "host0")
	dp21.Attributes().InsertString("direction", "read")
	dp21.Attributes().InsertString("device", "sda1")
	dp21.Attributes().Sort()
	dp21.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp21.SetIntVal(1e9)
	dp22 := m2.Sum().DataPoints().AppendEmpty()
	dp22.Attributes().InsertString("host", "host0")
	dp22.Attributes().InsertString("direction", "read")
	dp22.Attributes().InsertString("device", "sda2")
	dp22.Attributes().Sort()
	dp22.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp22.SetIntVal(2e9)
	dp23 := m2.Sum().DataPoints().AppendEmpty()
	dp23.Attributes().InsertString("host", "host0")
	dp23.Attributes().InsertString("direction", "write")
	dp23.Attributes().InsertString("device", "sda1")
	dp23.Attributes().Sort()
	dp23.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp23.SetIntVal(3e9)
	dp24 := m2.Sum().DataPoints().AppendEmpty()
	dp24.Attributes().InsertString("host", "host0")
	dp24.Attributes().InsertString("direction", "write")
	dp24.Attributes().InsertString("device", "sda2")
	dp24.Attributes().Sort()
	dp24.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp24.SetIntVal(8e9)

	m3 := ms.AppendEmpty()
	m3.SetName("system.disk.operations")
	m3.SetDescription("Disk operations count.")
	m3.SetUnit("bytes")
	m3.SetDataType(pmetric.MetricDataTypeSum)
	m3.Sum().SetIsMonotonic(true)
	m3.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	dp31 := m3.Sum().DataPoints().AppendEmpty()
	dp31.Attributes().InsertString("host", "host0")
	dp31.Attributes().InsertString("direction", "write")
	dp31.Attributes().InsertString("device", "sda1")
	dp31.Attributes().Sort()
	dp31.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp31.SetIntVal(4e3)
	dp32 := m3.Sum().DataPoints().AppendEmpty()
	dp32.Attributes().InsertString("host", "host0")
	dp32.Attributes().InsertString("direction", "read")
	dp32.Attributes().InsertString("device", "sda2")
	dp32.Attributes().Sort()
	dp32.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp32.SetIntVal(6e3)
	dp33 := m3.Sum().DataPoints().AppendEmpty()
	dp33.Attributes().InsertString("host", "host0")
	dp33.Attributes().InsertString("direction", "write")
	dp33.Attributes().InsertString("device", "sda1")
	dp33.Attributes().Sort()
	dp33.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp33.SetIntVal(1e3)
	dp34 := m3.Sum().DataPoints().AppendEmpty()
	dp34.Attributes().InsertString("host", "host0")
	dp34.Attributes().InsertString("direction", "write")
	dp34.Attributes().InsertString("device", "sda2")
	dp34.Attributes().Sort()
	dp34.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp34.SetIntVal(5e3)

	m4 := ms.AppendEmpty()
	m4.SetName("system.disk.operations")
	m4.SetDescription("Disk operations count.")
	m4.SetUnit("bytes")
	m4.SetDataType(pmetric.MetricDataTypeSum)
	m4.Sum().SetIsMonotonic(true)
	m4.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
	dp41 := m4.Sum().DataPoints().AppendEmpty()
	dp41.Attributes().InsertString("host", "host0")
	dp41.Attributes().InsertString("direction", "read")
	dp41.Attributes().InsertString("device", "sda1")
	dp41.Attributes().Sort()
	dp41.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp41.SetIntVal(6e3)
	dp42 := m4.Sum().DataPoints().AppendEmpty()
	dp42.Attributes().InsertString("host", "host0")
	dp42.Attributes().InsertString("direction", "read")
	dp42.Attributes().InsertString("device", "sda2")
	dp42.Attributes().Sort()
	dp42.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp42.SetIntVal(8e3)
	dp43 := m4.Sum().DataPoints().AppendEmpty()
	dp43.Attributes().InsertString("host", "host0")
	dp43.Attributes().InsertString("direction", "write")
	dp43.Attributes().InsertString("device", "sda1")
	dp43.Attributes().Sort()
	dp43.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp43.SetIntVal(3e3)
	dp44 := m4.Sum().DataPoints().AppendEmpty()
	dp44.Attributes().InsertString("host", "host0")
	dp44.Attributes().InsertString("direction", "write")
	dp44.Attributes().InsertString("device", "sda2")
	dp44.Attributes().Sort()
	dp44.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000060, 0)))
	dp44.SetIntVal(7e3)

	m5 := ms.AppendEmpty()
	m5.SetName("system.network.io")
	m5.SetDescription("The number of bytes transmitted and received")
	m5.SetUnit("bytes")
	m5.SetDataType(pmetric.MetricDataTypeGauge)
	dp51 := m5.Gauge().DataPoints().AppendEmpty()
	dp51.Attributes().InsertString("host", "host0")
	dp51.Attributes().InsertString("direction", "receive")
	dp51.Attributes().InsertString("device", "eth0")
	dp51.Attributes().InsertString("kubernetes_node", "node0")
	dp51.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp51.Attributes().Sort()
	dp51.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp51.SetIntVal(4e9)
	dp52 := m5.Gauge().DataPoints().AppendEmpty()
	dp52.Attributes().InsertString("host", "host0")
	dp52.Attributes().InsertString("direction", "transmit")
	dp52.Attributes().InsertString("device", "eth0")
	dp52.Attributes().InsertString("kubernetes_node", "node0")
	dp52.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp52.Attributes().Sort()
	dp52.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp52.SetIntVal(6e9)

	m6 := ms.AppendEmpty()
	m6.SetName("system.network.packets")
	m6.SetDescription("The number of packets transferred")
	m6.SetDataType(pmetric.MetricDataTypeGauge)
	dp61 := m6.Gauge().DataPoints().AppendEmpty()
	dp61.Attributes().InsertString("host", "host0")
	dp61.Attributes().InsertString("direction", "receive")
	dp61.Attributes().InsertString("device", "eth0")
	dp61.Attributes().InsertString("kubernetes_node", "node0")
	dp61.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp61.Attributes().Sort()
	dp61.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp61.SetIntVal(200)
	dp62 := m6.Gauge().DataPoints().AppendEmpty()
	dp62.Attributes().InsertString("host", "host0")
	dp62.Attributes().InsertString("direction", "receive")
	dp62.Attributes().InsertString("device", "eth1")
	dp62.Attributes().InsertString("kubernetes_node", "node0")
	dp62.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp62.Attributes().Sort()
	dp62.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp62.SetIntVal(150)

	m7 := ms.AppendEmpty()
	m7.SetName("container.memory.working_set")
	m7.SetUnit("bytes")
	m7.SetDataType(pmetric.MetricDataTypeGauge)
	dp71 := m7.Gauge().DataPoints().AppendEmpty()
	dp71.Attributes().InsertString("host", "host0")
	dp71.Attributes().InsertString("kubernetes_node", "node0")
	dp71.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp71.Attributes().Sort()
	dp71.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp71.SetIntVal(1000)

	m8 := ms.AppendEmpty()
	m8.SetName("container.memory.page_faults")
	m8.SetDataType(pmetric.MetricDataTypeGauge)
	dp81 := m8.Gauge().DataPoints().AppendEmpty()
	dp81.Attributes().InsertString("host", "host0")
	dp81.Attributes().InsertString("kubernetes_node", "node0")
	dp81.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp81.Attributes().Sort()
	dp81.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp81.SetIntVal(1000)

	m9 := ms.AppendEmpty()
	m9.SetName("container.memory.major_page_faults")
	m9.SetDataType(pmetric.MetricDataTypeGauge)
	dp91 := m9.Gauge().DataPoints().AppendEmpty()
	dp91.Attributes().InsertString("host", "host0")
	dp91.Attributes().InsertString("kubernetes_node", "node0")
	dp91.Attributes().InsertString("kubernetes_cluster", "cluster0")
	dp91.Attributes().Sort()
	dp91.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1596000000, 0)))
	dp91.SetIntVal(1000)

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
	rules, err := loadDefaultTranslationRules()
	require.NoError(t, err)
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

func TestDefaultExcludes_translated(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	setDefaultExcludes(cfg)

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
	setDefaultExcludes(cfg)

	converter, err := translation.NewMetricsConverter(zap.NewNop(), nil, cfg.ExcludeMetrics, cfg.IncludeMetrics, "")
	require.NoError(t, err)

	var metrics []map[string]string
	err = testReadJSON("testdata/json/non_default_metrics_otel_convention.json", &metrics)
	require.NoError(t, err)

	md := getMetrics(metrics)
	require.Equal(t, 65, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	dps := converter.MetricsToSignalFxV2(md)
	require.Equal(t, 0, len(dps))
}

func getMetrics(metrics []map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	ilms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilms.Metrics().EnsureCapacity(len(metrics))

	for _, mp := range metrics {
		m := ilms.Metrics().AppendEmpty()
		// Set data type to some arbitrary since it does not matter for this test.
		m.SetDataType(pmetric.MetricDataTypeSum)
		dp := m.Sum().DataPoints().AppendEmpty()
		dp.SetIntVal(0)
		attributesMap := dp.Attributes()
		for k, v := range mp {
			if v == "" {
				m.SetName(k)
				continue
			}
			attributesMap.InsertString(k, v)
		}
	}
	return md
}

func testReadJSON(f string, v interface{}) error {
	file, err := os.Open(f)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &v)
}
