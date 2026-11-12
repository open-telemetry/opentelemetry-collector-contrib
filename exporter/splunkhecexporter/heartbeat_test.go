// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
)

const (
	metricLabelKey = "customKey"
	metricLabelVal = "customVal"
)

func createTestConfig(metricsOverrides map[string]string, enableMetrics bool) *Config {
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.Heartbeat = HecHeartbeat{
		Interval: 10 * time.Millisecond,
	}
	config.Telemetry = HecTelemetry{
		Enabled:              enableMetrics,
		OverrideMetricsNames: metricsOverrides,
		ExtraAttributes: map[string]string{
			metricLabelKey: metricLabelVal,
		},
	}
	return config
}

func initHeartbeater(t *testing.T, metricsOverrides map[string]string, enableMetrics bool, consumeFn func(ctx context.Context, ld plog.Logs) error, mp *sdkmetric.MeterProvider) {
	config := createTestConfig(metricsOverrides, enableMetrics)
	tb, err := metadata.NewTelemetryBuilder(component.TelemetrySettings{MeterProvider: mp})
	require.NoError(t, err)
	hbter := newHeartbeater(config, component.NewDefaultBuildInfo(), consumeFn, tb, mp.Meter("test"), zap.NewNop())
	t.Cleanup(func() {
		hbter.shutdown()
	})
}

func assertHeartbeatInfoLog(t *testing.T, l plog.Logs) {
	assert.Equal(t, 1, l.ResourceLogs().Len())
	assert.Contains(t, l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString(), "HeartbeatInfo")
}

func getMetricValue(reader *sdkmetric.ManualReader, name string) ([]int64, error) {
	var md metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &md)
	var ret []int64
	for _, sm := range md.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				g := m.Data.(metricdata.Sum[int64])
				ret = append(ret, g.DataPoints[0].Value)
			}
		}
	}
	return ret, err
}

func getAttributes(reader *sdkmetric.ManualReader, name string) ([]attribute.Set, error) {
	var md metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &md)
	var ret []attribute.Set
	for _, sm := range md.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				g := m.Data.(metricdata.Sum[int64])
				ret = append(ret, g.DataPoints[0].Attributes)
			}
		}
	}
	return ret, err
}

func Test_newHeartbeater_disabled(t *testing.T) {
	config := createTestConfig(map[string]string{}, false)
	config.Heartbeat.Interval = 0
	tb, err := metadata.NewTelemetryBuilder(component.TelemetrySettings{MeterProvider: metricnoop.NewMeterProvider()})
	require.NoError(t, err)
	hb := newHeartbeater(config, component.NewDefaultBuildInfo(), func(_ context.Context, _ plog.Logs) error {
		return nil
	}, tb, metricnoop.NewMeterProvider().Meter("test"), zap.NewNop())
	assert.Nil(t, hb)
}

func Test_newHeartbeater_telemetryDeprecationWarning(t *testing.T) {
	tests := []struct {
		name        string
		telemetry   HecTelemetry
		wantWarning bool
	}{
		{
			name:        "unset telemetry block does not warn",
			telemetry:   HecTelemetry{},
			wantWarning: false,
		},
		{
			name:        "enabled telemetry warns",
			telemetry:   HecTelemetry{Enabled: true},
			wantWarning: true,
		},
		{
			name:        "override_metrics_names warns",
			telemetry:   HecTelemetry{OverrideMetricsNames: map[string]string{defaultHBSentMetricsName: "custom"}},
			wantWarning: true,
		},
		{
			name:        "extra_attributes warns",
			telemetry:   HecTelemetry{ExtraAttributes: map[string]string{"k": "v"}},
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewFactory().CreateDefaultConfig().(*Config)
			config.Heartbeat = HecHeartbeat{Interval: 10 * time.Millisecond}
			config.Telemetry = tt.telemetry

			core, logs := observer.New(zapcore.WarnLevel)
			tb, err := metadata.NewTelemetryBuilder(component.TelemetrySettings{MeterProvider: metricnoop.NewMeterProvider()})
			require.NoError(t, err)
			hbter := newHeartbeater(config, component.NewDefaultBuildInfo(), func(_ context.Context, _ plog.Logs) error {
				return nil
			}, tb, metricnoop.NewMeterProvider().Meter("test"), zap.New(core))
			t.Cleanup(hbter.shutdown)

			warnings := logs.FilterMessageSnippet("telemetry` configuration block").All()
			if tt.wantWarning {
				require.Len(t, warnings, 1)
			} else {
				require.Empty(t, warnings)
			}
		})
	}
}

func Test_Heartbeat_success(t *testing.T) {
	tests := []struct {
		name             string
		metricsOverrides map[string]string
		telemetryEnabled bool
	}{
		{
			// The metric defined in metadata.yaml is emitted even though the
			// deprecated telemetry.enabled flag is left at its default (false).
			name:             "default metric name, telemetry.enabled unset",
			metricsOverrides: map[string]string{},
			telemetryEnabled: false,
		},
		{
			name:             "default metric name, telemetry.enabled set",
			metricsOverrides: map[string]string{},
			telemetryEnabled: true,
		},
		{
			// The deprecated override is still honored for backwards compatibility.
			name: "overridden metric name honored for back-compat",
			metricsOverrides: map[string]string{
				defaultHBSentMetricsName: "app_heartbeat_success_total",
			},
			telemetryEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumeLogsChan := make(chan plog.Logs, 10)
			consumeFn := func(_ context.Context, ld plog.Logs) error {
				select {
				case consumeLogsChan <- ld:
				default:
				}
				return nil
			}
			reader := sdkmetric.NewManualReader()
			meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			initHeartbeater(t, tt.metricsOverrides, tt.telemetryEnabled, consumeFn, meterProvider)

			assert.Eventually(t, func() bool {
				return len(consumeLogsChan) != 0
			}, time.Second, 10*time.Millisecond)

			logs := <-consumeLogsChan
			assertHeartbeatInfoLog(t, logs)

			sentMetricsName := getMetricsName(tt.metricsOverrides, defaultHBSentMetricsName)
			var got []int64
			var err error
			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				got, err = getMetricValue(reader, sentMetricsName)
				require.NoError(c, err)
				if assert.NotEmpty(c, got) {
					assert.Positive(c, got[0], "there should be at least one success metric datapoint")
				}
			}, time.Second, 10*time.Millisecond)
			attrs, err := getAttributes(reader, sentMetricsName)
			require.NoError(t, err)
			assert.Equal(t, attribute.NewSet(attribute.String(metricLabelKey, metricLabelVal)), attrs[0])
		})
	}
}

func Test_Heartbeat_failure(t *testing.T) {
	consumeFn := func(_ context.Context, _ plog.Logs) error {
		return errors.New("always error")
	}
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	initHeartbeater(t, map[string]string{}, true, consumeFn, meterProvider)

	var got []int64
	var err error
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		got, err = getMetricValue(reader, defaultHBFailedMetricsName)
		require.NoError(tt, err)
		if assert.NotEmpty(tt, got) {
			assert.Positive(tt, got[0], "there should be at least one failure metric datapoint")
		}
	}, time.Second, 10*time.Millisecond)
	attrs, err := getAttributes(reader, defaultHBFailedMetricsName)
	require.NoError(t, err)
	assert.Equal(t, attribute.NewSet(attribute.String(metricLabelKey, metricLabelVal)), attrs[0])
}
