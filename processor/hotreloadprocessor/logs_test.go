// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/hotreloadprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/plog"
	otelprocessor "go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/processortest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
)

type componentTestTelemetry struct {
	reader        *sdkmetric.ManualReader
	meterProvider *sdkmetric.MeterProvider
}

func (tt *componentTestTelemetry) NewSettings() otelprocessor.Settings {
	settings := processortest.NewNopSettings(metadata.Type)
	settings.MeterProvider = tt.meterProvider
	settings.ID = component.NewID(metadata.Type)

	return settings
}

func (tt *componentTestTelemetry) assertMetrics(
	t *testing.T,
	expected []metricdata.Metrics,
	totalMetrics *int,
) {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &md))
	// ensure all required metrics are present
	for _, want := range expected {
		got := tt.getMetric(want.Name, md)
		require.Equal(t, want.Name, got.Name)
	}

	// ensure no additional metrics are emitted - ignore duration metrics
	if totalMetrics != nil {
		require.Equal(t, *totalMetrics, tt.len(md))
	}
}

func (tt *componentTestTelemetry) len(got metricdata.ResourceMetrics) int {
	metricsCount := 0
	for _, sm := range got.ScopeMetrics {
		metricsCount += len(sm.Metrics)
	}

	return metricsCount
}

func (tt *componentTestTelemetry) getMetric(
	name string,
	got metricdata.ResourceMetrics,
) metricdata.Metrics {
	for _, sm := range got.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	return metricdata.Metrics{}
}

func setupMetricsCollection() componentTestTelemetry {
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	return componentTestTelemetry{
		reader:        reader,
		meterProvider: meterProvider,
	}
}

func TestConsumeLogs(t *testing.T) {
	tests := []struct {
		name           string
		conf           func() string
		logs           func() plog.Logs
		wantLogs       func() string
		expectedResult []metricdata.Metrics
		totalMetrics   *int
	}{
		{
			name: `run pipeline`,
			conf: func() string {
				return readTestData(t, "config.yaml")
			},
			logs: func() plog.Logs {
				return readTestLogs(t, "logs.json")
			},
			wantLogs: func() string {
				return readWantedLogs(t, "logs_expected.json")
			},
			expectedResult: []metricdata.Metrics{{
				Name: "otelcol_processor_hot_reload_process_duration",
				Data: metricdata.Histogram[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_running_processors_count",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_newest_file_success_timestamp",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_reload_duration",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_scan",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_config_refresh_interval",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_config_shutdown_delay",
				Data: metricdata.Gauge[int64]{},
			}},
			totalMetrics: nil,
		},
		{
			name: `run pipeline without processors`,
			conf: func() string {
				return readTestData(t, "config-wo-processors.yaml")
			},
			logs: func() plog.Logs {
				return readTestLogs(t, "logs.json")
			},
			wantLogs: func() string {
				return readWantedLogs(t, "logs.json")
			},
			expectedResult: []metricdata.Metrics{{
				Name: "otelcol_processor_hot_reload_process_duration",
				Data: metricdata.Histogram[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_running_processors_count",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_newest_file_success_timestamp",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_reload_duration",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_scan",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_config_refresh_interval",
				Data: metricdata.Gauge[int64]{},
			}, {
				Name: "otelcol_processor_hot_reload_config_shutdown_delay",
				Data: metricdata.Gauge[int64]{},
			}},
			totalMetrics: ptrInt(7),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logs := test.logs()
			transformedLogs := plog.NewLogs()
			logsConsumer, _ := consumer.NewLogs(func(_ context.Context, ld plog.Logs) error {
				ld.CopyTo(transformedLogs)
				return nil
			})

			tel := setupMetricsCollection()
			hotreloadProcessor, err := newHotReloadLogsProcessor(
				context.Background(),
				otelprocessor.Settings{
					TelemetrySettings: component.TelemetrySettings{
						Logger:        zap.NewNop(),
						MeterProvider: tel.meterProvider,
					},
				},
				&Config{
					ConfigurationPrefix: test.conf(),
					EncryptionKey:       "test",
					Region:              "us-east-1",
					RefreshInterval:     60 * time.Second,
					ShutdownDelay:       10 * time.Second,
				},
				logsConsumer,
			)
			require.NoError(t, err)

			mockHost := newMockHost(t)
			err = hotreloadProcessor.Start(t.Context(), mockHost)
			require.NoError(t, err)

			err = hotreloadProcessor.ConsumeLogs(t.Context(), logs)
			require.NoError(t, err)

			err = hotreloadProcessor.Shutdown(t.Context())
			require.NoError(t, err)

			logsMarshaler := plog.JSONMarshaler{}
			logsMarshaler.MarshalLogs(transformedLogs)
			json, err := logsMarshaler.MarshalLogs(transformedLogs)
			require.NoError(t, err)
			require.JSONEq(t, test.wantLogs(), string(json))

			tel.assertMetrics(t, test.expectedResult, test.totalMetrics)
		})
	}
}

func TestRefreshConfig(t *testing.T) {
	tests := []struct {
		name           string
		conf           func() string
		logs           func() plog.Logs
		wantLogs       func() string
		expectedResult []metricdata.Metrics
	}{
		{
			name: `run pipeline         `,
			conf: func() string {
				return readTestData(t, "config.yaml")
			},
			logs: func() plog.Logs {
				return readTestLogs(t, "logs.json")
			},
			wantLogs: func() string {
				return readWantedLogs(t, "logs_expected.json")
			},
			expectedResult: []metricdata.Metrics{{
				Name:        "otelcol_processor_hot_reload_process_duration",
				Description: "Duration of the hotreload processor processing logs",
				Unit:        "ms",
				Data:        metricdata.Histogram[int64]{},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// logs := test.logs()
			transformedLogs := plog.NewLogs()
			logsConsumer, _ := consumer.NewLogs(func(_ context.Context, ld plog.Logs) error {
				ld.CopyTo(transformedLogs)
				return nil
			})

			tel := setupMetricsCollection()
			hotreloadProcessor, err := newHotReloadLogsProcessor(
				context.Background(),
				otelprocessor.Settings{
					TelemetrySettings: component.TelemetrySettings{
						Logger:        zap.NewExample(),
						MeterProvider: tel.meterProvider,
					},
				},
				&Config{
					ConfigurationPrefix: test.conf(),
					EncryptionKey:       "test",
					Region:              "us-east-1",
					RefreshInterval:     500 * time.Millisecond,
					ShutdownDelay:       10 * time.Second,
				},
				logsConsumer,
			)
			require.NoError(t, err)

			mockHost := newMockHost(t)
			err = hotreloadProcessor.Start(t.Context(), mockHost)
			require.NoError(t, err)

			oldSubprocessors := hotreloadProcessor.subprocessors.Load()
			time.Sleep(1200 * time.Millisecond)
			hotreloadProcessor.refreshConfig.Stop()
			newSubprocessors := hotreloadProcessor.subprocessors.Load()
			// asser pointer address is different
			require.NotEqual(t, oldSubprocessors, newSubprocessors)

			err = hotreloadProcessor.Shutdown(t.Context())
			require.NoError(t, err)
		})
	}
}

func readTestData(t *testing.T, file string) string {
	content, err := os.ReadFile(filepath.Join("testdata", file))
	require.NoError(t, err)
	require.NotEmpty(t, content)

	encoded := base64.StdEncoding.EncodeToString(content)
	return "data://" + encoded
}

func readTestLogs(t *testing.T, file string) plog.Logs {
	content, err := os.ReadFile(filepath.Join("testdata", file))
	require.NoError(t, err)
	require.NotEmpty(t, content)

	logsUnmarshaler := &plog.JSONUnmarshaler{}
	inputLogs, err := logsUnmarshaler.UnmarshalLogs([]byte(content))
	require.NoError(t, err)

	return inputLogs
}

func readWantedLogs(t *testing.T, file string) string {
	content, err := os.ReadFile(filepath.Join("testdata", file))
	require.NoError(t, err)
	require.NotEmpty(t, content)

	return string(content)
}

func ptrInt(i int) *int {
	return &i
}

// mockHost provides a mock component.Host with processor factories for testing
type mockHost struct {
	component.Host
	factories otelcol.Factories
}

func newMockHost(t *testing.T) component.Host {
	factories, err := otelcol.MakeFactoryMap[otelprocessor.Factory](
		transformprocessor.NewFactory(),
		batchprocessor.NewFactory(),
	)
	require.NoError(t, err)
	return &mockHost{
		factories: otelcol.Factories{
			Processors: factories,
		},
	}
}

func (m *mockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	if kind == component.KindProcessor {
		return m.factories.Processors[componentType]
	}
	return nil
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return nil
}
