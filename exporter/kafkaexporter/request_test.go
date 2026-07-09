// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
)

// setUseRequestTypeFeatureGate toggles the exporter.kafka.useRequestType gate
// for the duration of the test, restoring its previous value on cleanup.
func setUseRequestTypeFeatureGate(tb testing.TB, enabled bool) {
	prev := metadata.ExporterKafkaUseRequestTypeFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(metadata.ExporterKafkaUseRequestTypeFeatureGate.ID(), enabled))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(metadata.ExporterKafkaUseRequestTypeFeatureGate.ID(), prev))
	})
}

func TestRequestRoundTrip_Traces(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, cluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), config.Traces.Topic)

	req, err := newRequestConverter(exp)(t.Context(), testdata.GenerateTraces(2))
	require.NoError(t, err)
	require.NoError(t, newRequestPusher(exp)(t.Context(), req))

	records := fetchKgoRecords(t, cluster.ListenAddrs(), config.Traces.Topic, 1)
	require.Len(t, records, 1)
	assert.NotEmpty(t, records[0].Value)
}

func TestRequestRoundTrip_Logs(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, cluster := newKgoMockLogsExporter(t, *config, componenttest.NewNopHost(), config.Logs.Topic)

	req, err := newRequestConverter(exp)(t.Context(), testdata.GenerateLogs(2))
	require.NoError(t, err)
	require.NoError(t, newRequestPusher(exp)(t.Context(), req))

	records := fetchKgoRecords(t, cluster.ListenAddrs(), config.Logs.Topic, 1)
	require.Len(t, records, 1)
	assert.NotEmpty(t, records[0].Value)
}

func TestRequestRoundTrip_Metrics(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, cluster := newKgoMockMetricsExporter(t, *config, componenttest.NewNopHost(), config.Metrics.Topic)

	req, err := newRequestConverter(exp)(t.Context(), testdata.GenerateMetrics(2))
	require.NoError(t, err)
	require.NoError(t, newRequestPusher(exp)(t.Context(), req))

	records := fetchKgoRecords(t, cluster.ListenAddrs(), config.Metrics.Topic, 1)
	require.Len(t, records, 1)
	assert.NotEmpty(t, records[0].Value)
}

func TestRequestRoundTrip_Profiles(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, cluster := newKgoMockProfilesExporter(t, *config, componenttest.NewNopHost(), config.Profiles.Topic)

	req, err := newRequestConverter(exp)(t.Context(), testdata.GenerateProfiles(2))
	require.NoError(t, err)
	require.NoError(t, newRequestPusher(exp)(t.Context(), req))

	records := fetchKgoRecords(t, cluster.ListenAddrs(), config.Profiles.Topic, 1)
	require.Len(t, records, 1)
	assert.NotEmpty(t, records[0].Value)
}

func TestRequestConverter_NotStarted(t *testing.T) {
	exp := newTracesExporter(*createDefaultConfig().(*Config), exportertest.NewNopSettings(metadata.Type))
	_, err := newRequestConverter(exp)(t.Context(), testdata.GenerateTraces(1))
	assert.Error(t, err)
}

func TestRequestPusher_NotStarted(t *testing.T) {
	exp := newTracesExporter(*createDefaultConfig().(*Config), exportertest.NewNopSettings(metadata.Type))
	err := newRequestPusher(exp)(t.Context(), fakeRequest{})
	assert.Error(t, err)
}

func TestRequestPusher_WrongRequestType(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, _ := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), config.Traces.Topic)
	err := newRequestPusher(exp)(t.Context(), fakeRequest{})
	assert.Error(t, err)
}

// fakeRequest is an arbitrary xexporterhelper.Request implementation used to
// exercise the type-assertion failure path in the request pusher.
type fakeRequest struct{}

func (fakeRequest) ItemsCount() int { return 0 }
func (fakeRequest) BytesSize() int  { return 0 }
func (fakeRequest) MergeSplit(context.Context, int, exporterhelper.RequestSizerType, xexporterhelper.Request) ([]xexporterhelper.Request, error) {
	return nil, nil
}

func TestCreateExporters_RequestTypeGateEnabled(t *testing.T) {
	setUseRequestTypeFeatureGate(t, true)

	conf := applyConfigOption(func(conf *Config) {
		conf.Metadata.Full = false
		conf.Brokers = []string{"invalid:9092"}
	})
	f := NewFactory()

	t.Run("traces", func(t *testing.T) {
		exp, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), conf)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, exp.Shutdown(t.Context()))
	})
	t.Run("metrics", func(t *testing.T) {
		exp, err := f.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), conf)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, exp.Shutdown(t.Context()))
	})
	t.Run("logs", func(t *testing.T) {
		exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), conf)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, exp.Shutdown(t.Context()))
	})
	t.Run("profiles", func(t *testing.T) {
		exp, err := f.(xexporter.Factory).CreateProfiles(t.Context(), exportertest.NewNopSettings(metadata.Type), conf)
		require.NoError(t, err)
		require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, exp.Shutdown(t.Context()))
	})
}
