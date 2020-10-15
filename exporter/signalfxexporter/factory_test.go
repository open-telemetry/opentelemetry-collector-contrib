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

package signalfxexporter

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	_, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, cfg)
	assert.NoError(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	c := cfg.(*Config)
	c.AccessToken = "access_token"
	c.Realm = "us0"

	exp, err := factory.CreateMetricsExporter(
		context.Background(),
		component.ExporterCreateParams{Logger: zap.NewNop()},
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	expCfg := cfg.(*Config)
	expCfg.AccessToken = "testToken"
	expCfg.Realm = "us1"
	exp, err = factory.CreateMetricsExporter(
		context.Background(),
		component.ExporterCreateParams{Logger: zap.NewNop()},
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	logExp, err := factory.CreateLogsExporter(
		context.Background(),
		component.ExporterCreateParams{Logger: zap.NewNop()},
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, logExp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestCreateMetricsExporter_CustomConfig(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		AccessToken: "testToken",
		Realm:       "us1",
		Headers: map[string]string{
			"added-entry": "added value",
			"dot.test":    "test",
		},
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 2 * time.Second},
	}

	te, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, config)
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
				ExporterSettings: configmodels.ExporterSettings{
					TypeVal: configmodels.Type(typeStr),
					NameVal: typeStr,
				},
				AccessToken:     "testToken",
				Realm:           "lab",
				TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: -2 * time.Second},
			},
			errorMessage: "failed to process \"signalfx\" config: cannot have a negative \"timeout\"",
		},
		{
			name: "empty_realm_and_urls",
			config: &Config{
				ExporterSettings: configmodels.ExporterSettings{
					TypeVal: configmodels.Type(typeStr),
					NameVal: typeStr,
				},
				AccessToken: "testToken",
			},
			errorMessage: "failed to process \"signalfx\" config: requires a non-empty \"realm\"," +
				" or \"ingest_url\" and \"api_url\" should be explicitly set",
		},
		{
			name: "empty_realm_and_api_url",
			config: &Config{
				ExporterSettings: configmodels.ExporterSettings{
					TypeVal: configmodels.Type(typeStr),
					NameVal: typeStr,
				},
				AccessToken: "testToken",
				IngestURL:   "http://localhost:123",
			},
			errorMessage: "failed to process \"signalfx\" config: requires a non-empty \"realm\"," +
				" or \"ingest_url\" and \"api_url\" should be explicitly set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, tt.config)
			assert.EqualError(t, err, tt.errorMessage)
			assert.Nil(t, te)
		})
	}
}

func TestCreateMetricsExporterWithDefaultTranslaitonRules(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		AccessToken:           "testToken",
		Realm:                 "us1",
		SendCompatibleMetrics: true,
	}

	te, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)

	// Validate that default translation rules are loaded
	// Expected values has to be updated once default config changed
	assert.Equal(t, 51, len(config.TranslationRules))
	assert.Equal(t, translation.ActionRenameDimensionKeys, config.TranslationRules[0].Action)
	assert.Equal(t, 33, len(config.TranslationRules[0].Mapping))
}

func TestCreateMetricsExporterWithSpecifiedTranslaitonRules(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		AccessToken:           "testToken",
		Realm:                 "us1",
		SendCompatibleMetrics: true,
		TranslationRules: []translation.Rule{
			{
				Action: translation.ActionRenameDimensionKeys,
				Mapping: map[string]string{
					"old_dimension": "new_dimension",
				},
			},
		},
	}

	te, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)

	// Validate that specified translation rules are loaded instead of default ones
	assert.Equal(t, 1, len(config.TranslationRules))
	assert.Equal(t, translation.ActionRenameDimensionKeys, config.TranslationRules[0].Action)
	assert.Equal(t, 1, len(config.TranslationRules[0].Mapping))
	assert.Equal(t, "new_dimension", config.TranslationRules[0].Mapping["old_dimension"])
}

func TestCreateMetricsExporterWithExcludedMetrics(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		AccessToken:    "testToken",
		Realm:          "us1",
		ExcludeMetrics: []string{"metric1"},
	}

	te, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, config)
	require.NoError(t, err)
	require.NotNil(t, te)

	assert.Equal(t, 1, len(config.TranslationRules))
	assert.Equal(t, translation.ActionDropMetrics, config.TranslationRules[0].Action)
	assert.Equal(t, 1, len(config.TranslationRules[0].MetricNames))
	assert.True(t, config.TranslationRules[0].MetricNames["metric1"])
}

func TestCreateMetricsExporterWithDefinedRulesAndExcludedMetrics(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		AccessToken: "testToken",
		Realm:       "us1",
		TranslationRules: []translation.Rule{
			{
				Action: translation.ActionRenameDimensionKeys,
				Mapping: map[string]string{
					"old_dimension": "new_dimension",
				},
			},
		},
		ExcludeMetrics: []string{"metric1"},
	}

	te, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, config)
	require.NoError(t, err)
	require.NotNil(t, te)

	assert.Equal(t, 2, len(config.TranslationRules))
	assert.Equal(t, translation.ActionRenameDimensionKeys, config.TranslationRules[0].Action)
	assert.Equal(t, translation.ActionDropMetrics, config.TranslationRules[1].Action)
	assert.Equal(t, 1, len(config.TranslationRules[1].MetricNames))
	assert.True(t, config.TranslationRules[1].MetricNames["metric1"])
}

func TestDefaultTranslationRules(t *testing.T) {
	rules, err := loadDefaultTranslationRules()
	require.NoError(t, err)
	require.NotNil(t, rules, "rules are nil")
	tr, err := translation.NewMetricTranslator(rules, 1)
	require.NoError(t, err)
	data := testMetricsData()

	c := translation.NewMetricsConverter(zap.NewNop(), tr)
	translated, _ := c.MetricDataToSignalFxV2(data, nil)
	require.NotNil(t, translated)

	metrics := make(map[string][]*sfxpb.DataPoint)
	for _, pt := range translated {
		if _, ok := metrics[pt.Metric]; !ok {
			metrics[pt.Metric] = make([]*sfxpb.DataPoint, 0, 1)
		}
		metrics[pt.Metric] = append(metrics[pt.Metric], pt)
	}

	// memory.utilization new metric calculation
	dps, ok := metrics["memory.utilization"]
	require.True(t, ok, "memory.utilization metric not found")
	require.Equal(t, 1, len(dps))
	require.Equal(t, 40.0, *dps[0].Value.DoubleValue)

	// system.disk.ops metric split and dimension rename
	dps, ok = metrics["disk_ops.read"]
	require.True(t, ok, "disk_ops.read metrics not found")
	require.Equal(t, 4, len(dps))
	require.Equal(t, int64(4e3), *dps[0].Value.IntValue)
	require.Equal(t, "disk", dps[0].Dimensions[1].Key)
	require.Equal(t, "sda1", dps[0].Dimensions[1].Value)
	require.Equal(t, int64(6e3), *dps[1].Value.IntValue)
	require.Equal(t, "disk", dps[1].Dimensions[1].Key)
	require.Equal(t, "sda2", dps[1].Dimensions[1].Value)

	dps, ok = metrics["disk_ops.write"]
	require.True(t, ok, "disk_ops.write metrics not found")
	require.Equal(t, 4, len(dps))
	require.Equal(t, int64(1e3), *dps[0].Value.IntValue)
	require.Equal(t, "disk", dps[0].Dimensions[1].Key)
	require.Equal(t, "sda1", dps[0].Dimensions[1].Value)
	require.Equal(t, int64(5e3), *dps[1].Value.IntValue)
	require.Equal(t, "disk", dps[1].Dimensions[1].Key)
	require.Equal(t, "sda2", dps[1].Dimensions[1].Value)

	// disk_ops.total gauge from system.disk.ops cumulative, where is disk_ops.total
	// is the cumulative across devices and directions.
	dps, ok = metrics["disk_ops.total"]
	require.True(t, ok, "disk_ops.total metrics not found")
	require.Equal(t, 1, len(dps))
	require.Equal(t, int64(8e3), *dps[0].Value.IntValue)
	require.Equal(t, 1, len(dps[0].Dimensions))
	require.Equal(t, "host", dps[0].Dimensions[0].Key)
	require.Equal(t, "host0", dps[0].Dimensions[0].Value)

	// network.total new metric calculation
	dps, ok = metrics["network.total"]
	require.True(t, ok, "network.total metrics not found")
	require.Equal(t, 1, len(dps))
	require.Equal(t, 3, len(dps[0].Dimensions))
	require.Equal(t, int64(10e9), *dps[0].Value.IntValue)

	// memory page faults and working set renames
	_, ok = metrics["container_memory_working_set_bytes"]
	require.True(t, ok, "container_memory_working_set_bytes not found")
	_, ok = metrics["container_memory_page_faults"]
	require.True(t, ok, "container_memory_page_faults not found")
	_, ok = metrics["container_memory_major_page_faults"]
	require.True(t, ok, "container_memory_major_page_faults not found")
}

func testMetricsData() []consumerdata.MetricsData {
	md := consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "system.memory.usage",
					Description: "Bytes of memory in use",
					Unit:        "bytes",
					Type:        metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "state"},
						{Key: "host"},
						{Key: "kubernetes_node"},
						{Key: "kubernetes_cluster"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "used",
							HasValue: true,
						}, {
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 4e9,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "free",
							HasValue: true,
						}, {
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 6e9,
							},
						}},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "system.disk.ops",
					Description: "Disk operations count.",
					Unit:        "bytes",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "host"},
						{Key: "direction"},
						{Key: "device"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "read",
							HasValue: true,
						}, {
							Value:    "sda1",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 4e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "read",
							HasValue: true,
						}, {
							Value:    "sda2",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 6e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "write",
							HasValue: true,
						}, {
							Value:    "sda1",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 1e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "write",
							HasValue: true,
						}, {
							Value:    "sda2",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 5e3,
							},
						}},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "system.disk.ops",
					Description: "Disk operations count.",
					Unit:        "bytes",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "host"},
						{Key: "direction"},
						{Key: "device"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "read",
							HasValue: true,
						}, {
							Value:    "sda1",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000060,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 6e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "read",
							HasValue: true,
						}, {
							Value:    "sda2",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000060,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 8e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "write",
							HasValue: true,
						}, {
							Value:    "sda1",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000060,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 3e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "write",
							HasValue: true,
						}, {
							Value:    "sda2",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000060,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 7e3,
							},
						}},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "system.network.io",
					Description: "The number of bytes transmitted and received",
					Unit:        "bytes",
					Type:        metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "direction"},
						{Key: "interface"},
						{Key: "host"},
						{Key: "kubernetes_node"},
						{Key: "kubernetes_cluster"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "receive",
							HasValue: true,
						}, {
							Value:    "eth0",
							HasValue: true,
						}, {
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 4e9,
							},
						}},
					},
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "transmit",
							HasValue: true,
						}, {
							Value:    "eth0",
							HasValue: true,
						}, {
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 6e9,
							},
						}},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "container.memory.working_set",
					Unit: "bytes",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "host"},
						{Key: "kubernetes_node"},
						{Key: "kubernetes_cluster"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 1000,
							},
						}},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "container.memory.page_faults",
					Unit: "",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "host"},
						{Key: "kubernetes_node"},
						{Key: "kubernetes_cluster"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 1000,
							},
						}},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "container.memory.major_page_faults",
					Unit: "",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "host"},
						{Key: "kubernetes_node"},
						{Key: "kubernetes_cluster"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamppb.Timestamp{},
						LabelValues: []*metricspb.LabelValue{{
							Value:    "host0",
							HasValue: true,
						}, {
							Value:    "node0",
							HasValue: true,
						}, {
							Value:    "cluster0",
							HasValue: true,
						}},
						Points: []*metricspb.Point{{
							Timestamp: &timestamppb.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 1000,
							},
						}},
					},
				},
			},
		},
	}
	return []consumerdata.MetricsData{md}
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
