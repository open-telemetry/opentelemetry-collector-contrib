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
	"github.com/golang/protobuf/ptypes/timestamp"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"

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
		Timeout: 2 * time.Second,
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
				AccessToken: "testToken",
				Realm:       "lab",
				Timeout:     -2 * time.Second,
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
	assert.Equal(t, 33, len(config.TranslationRules))
	assert.Equal(t, translation.ActionRenameDimensionKeys, config.TranslationRules[0].Action)
	assert.Equal(t, 32, len(config.TranslationRules[0].Mapping))
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

func TestDefaultTranslationRules(t *testing.T) {
	rules, err := loadDefaultTranslationRules()
	require.NoError(t, err)
	require.NotNil(t, rules, "rules are nil")
	tr, err := translation.NewMetricTranslator(rules)
	require.NoError(t, err)
	data := md()

	translated, _ := translation.MetricDataToSignalFxV2(zap.NewNop(), tr, data)
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
	require.Equal(t, 2, len(dps))
	require.Equal(t, int64(4e3), *dps[0].Value.IntValue)
	require.Equal(t, "disk", dps[0].Dimensions[1].Key)
	require.Equal(t, "sda1", dps[0].Dimensions[1].Value)
	require.Equal(t, int64(6e3), *dps[1].Value.IntValue)
	require.Equal(t, "disk", dps[1].Dimensions[1].Key)
	require.Equal(t, "sda2", dps[1].Dimensions[1].Value)

	dps, ok = metrics["disk_ops.write"]
	require.True(t, ok, "disk_ops.write metrics not found")
	require.Equal(t, 2, len(dps))
	require.Equal(t, int64(1e3), *dps[0].Value.IntValue)
	require.Equal(t, "disk", dps[0].Dimensions[1].Key)
	require.Equal(t, "sda1", dps[0].Dimensions[1].Value)
	require.Equal(t, int64(5e3), *dps[1].Value.IntValue)
	require.Equal(t, "disk", dps[1].Dimensions[1].Key)
	require.Equal(t, "sda2", dps[1].Dimensions[1].Value)

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

func md() consumerdata.MetricsData {
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
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 4e9,
							},
						}},
					},
					{
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
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
					Type:        metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "host"},
						{Key: "direction"},
						{Key: "device"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 4e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 6e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 1e3,
							},
						}},
					},
					{
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
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
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
								Seconds: 1596000000,
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 4e9,
							},
						}},
					},
					{
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
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
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
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
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
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
						StartTimestamp: &timestamp.Timestamp{},
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
							Timestamp: &timestamp.Timestamp{
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
	return md
}

func TestDefaultDiskTranslations(t *testing.T) {
	file, err := os.Open("testdata/json/system.filesystem.usage.json")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	bytes, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	var pts []*sfxpb.DataPoint
	err = json.Unmarshal(bytes, &pts)
	require.NoError(t, err)

	rules, err := loadDefaultTranslationRules()
	require.NoError(t, err)
	require.NotNil(t, rules, "rules are nil")
	tr, err := translation.NewMetricTranslator(rules)
	require.NoError(t, err)

	translated := tr.TranslateDataPoints(zap.NewNop(), pts)
	require.NotNil(t, translated)

	m := map[string][]*sfxpb.DataPoint{}
	for _, pt := range translated {
		l := m[pt.Metric]
		l = append(l, pt)
		m[pt.Metric] = l
	}

	dtPts := m["disk.total"]
	require.Equal(t, 4, len(dtPts))
	require.Equal(t, 4, len(dtPts[0].Dimensions))

	dstPts := m["disk.summary_total"]
	require.Equal(t, 1, len(dstPts))
	require.Equal(t, 3, len(dstPts[0].Dimensions))

	utPts, ok := m["df_complex.used_total"]
	require.True(t, ok)
	require.Equal(t, 1, len(utPts))
	require.Equal(t, 3, len(utPts[0].Dimensions))
}
