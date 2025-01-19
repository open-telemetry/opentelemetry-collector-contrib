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
	hbter := newHeartbeater(config, component.NewDefaultBuildInfo(), consumeFn, mp.Meter("test"))
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
	hb := newHeartbeater(config, component.NewDefaultBuildInfo(), func(_ context.Context, _ plog.Logs) error {
		return nil
	}, metricnoop.NewMeterProvider().Meter("test"))
	assert.Nil(t, hb)
}

func Test_Heartbeat_success(t *testing.T) {
	tests := []struct {
		metricsOverrides map[string]string
		enableMetrics    bool
	}{
		{
			metricsOverrides: map[string]string{},
			enableMetrics:    false,
		},
		{
			metricsOverrides: map[string]string{
				defaultHBSentMetricsName: "app_heartbeat_success_total",
			},
			enableMetrics: true,
		},
		{
			metricsOverrides: map[string]string{},
			enableMetrics:    false,
		},
	}

	for _, tt := range tests {
		consumeLogsChan := make(chan plog.Logs, 10)
		consumeFn := func(_ context.Context, ld plog.Logs) error {
			consumeLogsChan <- ld
			return nil
		}
		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		initHeartbeater(t, tt.metricsOverrides, true, consumeFn, meterProvider)

		assert.Eventually(t, func() bool {
			return len(consumeLogsChan) != 0
		}, time.Second, 10*time.Millisecond)

		logs := <-consumeLogsChan
		assertHeartbeatInfoLog(t, logs)

		if tt.enableMetrics {
			sentMetricsName := getMetricsName(tt.metricsOverrides, defaultHBSentMetricsName)
			var got []int64
			var err error
			assert.Eventually(t, func() bool {
				got, err = getMetricValue(reader, sentMetricsName)
				require.NoError(t, err)
				return len(got) != 0
			}, time.Second, 10*time.Millisecond)
			assert.Positive(t, got[0], "there should be at least one success metric datapoint")
			attrs, err := getAttributes(reader, sentMetricsName)
			require.NoError(t, err)
			assert.Equal(t, attribute.NewSet(attribute.String(metricLabelKey, metricLabelVal)), attrs[0])
		}
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
	assert.Eventually(t, func() bool {
		got, err = getMetricValue(reader, defaultHBFailedMetricsName)
		require.NoError(t, err)
		return len(got) != 0
	}, time.Second, 10*time.Millisecond)
	assert.Positive(t, got[0], "there should be at least one failure metric datapoint")
	attrs, err := getAttributes(reader, defaultHBFailedMetricsName)
	require.NoError(t, err)
	assert.Equal(t, attribute.NewSet(attribute.String(metricLabelKey, metricLabelVal)), attrs[0])
}
