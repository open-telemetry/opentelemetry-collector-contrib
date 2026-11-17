// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

package remoteconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestAgentServiceMetricsReachCollectorProvider(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(t.Context())) })
	set := extension.Settings{ID: component.NewIDWithName(Type, "test"), TelemetrySettings: componenttest.NewNopTelemetrySettings()}
	set.MeterProvider = provider
	reporter, err := newServiceReporter(set)
	require.NoError(t, err)
	reporter.IncRateLimit()
	reporter.IncTimeout()
	reporter.IncConfigSubscriptionsConnectedCounter()
	reporter.IncConfigSubscriptionsDisconnectedCounter()
	reporter.SetConfigSubscriptionsActive(2)
	reporter.SetConfigSubscriptionClientsTracked(3)
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &collected))
	require.Len(t, collected.ScopeMetrics, 1)
	values := map[string]int64{}
	for _, m := range collected.ScopeMetrics[0].Metrics {
		switch data := m.Data.(type) {
		case metricdata.Sum[int64]:
			require.Len(t, data.DataPoints, 1)
			values[m.Name] = data.DataPoints[0].Value
		case metricdata.Gauge[int64]:
			require.Len(t, data.DataPoints, 1)
			values[m.Name] = data.DataPoints[0].Value
		default:
			t.Fatalf("unexpected RC metric type %T", data)
		}
	}
	require.Equal(t, map[string]int64{
		"datadog.remote_config.cache_bypass.rate_limited":  1,
		"datadog.remote_config.cache_bypass.timed_out":     1,
		"datadog.remote_config.subscriptions.connected":    1,
		"datadog.remote_config.subscriptions.disconnected": 1,
		"datadog.remote_config.subscriptions.active":       2,
		"datadog.remote_config.subscriptions.clients":      3,
	}, values)
}
