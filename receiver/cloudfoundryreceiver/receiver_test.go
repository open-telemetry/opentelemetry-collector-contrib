// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"context"
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver/internal/metadata"
)

// Test to make sure a new metrics receiver can be created properly, started and shutdown with the default config
func TestDefaultValidMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopSettings()

	receiver, err := newCloudFoundryMetricsReceiver(
		params,
		*cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	// Test start
	ctx := context.Background()
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

// Test to make sure a new logs receiver can be created properly, started and shutdown with the default config
func TestDefaultValidLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopSettings()

	receiver, err := newCloudFoundryLogsReceiver(
		params,
		*cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	// Test start
	ctx := context.Background()
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

func TestSetupMetricsScope(t *testing.T) {
	resourceSetup := pmetric.NewResourceMetrics()
	scope := resourceSetup.ScopeMetrics().AppendEmpty()
	scope.Scope().SetName(metadata.ScopeName)

	tests := []struct {
		name string
		in   pmetric.ResourceMetrics
		out  pmetric.ResourceMetrics
	}{
		{
			name: "scope empty",
			in:   pmetric.NewResourceMetrics(),
			out:  resourceSetup,
		},
		{
			name: "scope set",
			in:   resourceSetup,
			out:  resourceSetup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupMetricsScope(tt.in)
			require.Equal(t, tt.out, tt.in)
		})
	}
}

func TestGetResourceMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resource := metrics.ResourceMetrics().AppendEmpty()
	resource.Resource().Attributes().PutStr("org.cloudfoundry.origin", "rep")

	resource2 := pmetric.NewResourceMetrics()
	resource2.Resource().Attributes().PutStr("org.cloudfoundry.origin", "rep2")

	tests := []struct {
		name          string
		resourceAttrs bool
		envelope      loggregator_v2.Envelope
		metrics       pmetric.Metrics
		expected      pmetric.ResourceMetrics
	}{
		{
			name:          "resource attributes false",
			envelope:      loggregator_v2.Envelope{},
			resourceAttrs: false,
			metrics:       pmetric.NewMetrics(),
			expected:      pmetric.NewResourceMetrics(),
		},
		{
			name:          "empty metrics",
			envelope:      loggregator_v2.Envelope{},
			resourceAttrs: true,
			metrics:       pmetric.NewMetrics(),
			expected:      pmetric.NewResourceMetrics(),
		},
		{
			name: "matched resource metrics",
			envelope: loggregator_v2.Envelope{
				Tags: map[string]string{
					"origin": "rep",
					"custom": "datapoint",
				},
			},
			resourceAttrs: true,
			metrics:       metrics,
			expected:      resource,
		},
		{
			name: "non-matched resource metrics",
			envelope: loggregator_v2.Envelope{
				Tags: map[string]string{
					"origin": "rep2",
					"custom": "datapoint",
				},
			},
			resourceAttrs: true,
			metrics:       metrics,
			expected:      resource2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.resourceAttrs {
				require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), true))
				t.Cleanup(func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), false))
				})
			}
			require.Equal(t, tt.expected, getResourceMetrics(tt.metrics, &tt.envelope))
		})
	}
}

func TestSetupLogsScope(t *testing.T) {
	resourceSetup := plog.NewResourceLogs()
	scope := resourceSetup.ScopeLogs().AppendEmpty()
	scope.Scope().SetName(metadata.ScopeName)

	tests := []struct {
		name string
		in   plog.ResourceLogs
		out  plog.ResourceLogs
	}{
		{
			name: "scope empty",
			in:   plog.NewResourceLogs(),
			out:  resourceSetup,
		},
		{
			name: "scope set",
			in:   resourceSetup,
			out:  resourceSetup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLogsScope(tt.in)
			require.Equal(t, tt.out, tt.in)
		})
	}
}

func TestGetResourceLogs(t *testing.T) {
	logs := plog.NewLogs()
	resource := logs.ResourceLogs().AppendEmpty()
	resource.Resource().Attributes().PutStr("org.cloudfoundry.origin", "rep")

	resource2 := plog.NewResourceLogs()
	resource2.Resource().Attributes().PutStr("org.cloudfoundry.origin", "rep2")

	tests := []struct {
		name          string
		resourceAttrs bool
		envelope      loggregator_v2.Envelope
		logs          plog.Logs
		expected      plog.ResourceLogs
	}{
		{
			name:          "resource attributes false",
			envelope:      loggregator_v2.Envelope{},
			resourceAttrs: false,
			logs:          plog.NewLogs(),
			expected:      plog.NewResourceLogs(),
		},
		{
			name:          "empty logs",
			envelope:      loggregator_v2.Envelope{},
			resourceAttrs: true,
			logs:          plog.NewLogs(),
			expected:      plog.NewResourceLogs(),
		},
		{
			name: "matched resource logs",
			envelope: loggregator_v2.Envelope{
				Tags: map[string]string{
					"origin": "rep",
					"custom": "datapoint",
				},
			},
			resourceAttrs: true,
			logs:          logs,
			expected:      resource,
		},
		{
			name: "non-matched resource logs",
			envelope: loggregator_v2.Envelope{
				Tags: map[string]string{
					"origin": "rep2",
					"custom": "datapoint",
				},
			},
			resourceAttrs: true,
			logs:          logs,
			expected:      resource2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.resourceAttrs {
				require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), true))
				t.Cleanup(func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), false))
				})
			}
			require.Equal(t, tt.expected, getResourceLogs(tt.logs, &tt.envelope))
		})
	}
}

func TestBuildLogsWithResourceAttrs(t *testing.T) {
	logs := plog.NewLogs()
	observedTime := time.Date(2022, time.July, 14, 9, 30, 0, 0, time.UTC)

	require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), false))
	})

	// adding the first item to the logs

	envelope := &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom":     "datapoint",
		},
	}

	buildLogs(logs, envelope, observedTime)

	// check if the first log resource was created successfully
	// we await single resource, with single scope and single log
	require.Equal(t, 1, logs.ResourceLogs().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, logs.ResourceLogs().At(0).Resource().Attributes().AsRaw())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().Len())
	require.Equal(t, metadata.ScopeName, logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom": "datapoint",
	}, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())

	// adding another envelope matching the same resource attributes

	envelope = &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom2":    "datapoint2",
		},
	}

	buildLogs(logs, envelope, observedTime)

	// check that the first log resource contains a single scope, but 2 logs
	require.Equal(t, 1, logs.ResourceLogs().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, logs.ResourceLogs().At(0).Resource().Attributes().AsRaw())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().Len())
	require.Equal(t, metadata.ScopeName, logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	require.Equal(t, 2, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom": "datapoint",
	}, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom2": "datapoint2",
	}, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes().AsRaw())

	// adding another envelope not matching the resource attributes

	envelope = &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa33",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom3":    "datapoint3",
		},
	}

	buildLogs(logs, envelope, observedTime)

	// check that the new resource was created and exists next to the original one
	require.Equal(t, 2, logs.ResourceLogs().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa33",
	}, logs.ResourceLogs().At(1).Resource().Attributes().AsRaw())
	require.Equal(t, 1, logs.ResourceLogs().At(1).ScopeLogs().Len())
	require.Equal(t, metadata.ScopeName, logs.ResourceLogs().At(1).ScopeLogs().At(0).Scope().Name())
	require.Equal(t, 1, logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom3": "datapoint3",
	}, logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())
}

func TestBuildLogs(t *testing.T) {
	logs := plog.NewLogs()
	observedTime := time.Date(2022, time.July, 14, 9, 30, 0, 0, time.UTC)

	// adding the first item to the logs

	envelope := &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom":     "datapoint",
		},
	}

	buildLogs(logs, envelope, observedTime)

	// check if the first log resource was created successfully
	// we await single resource, with single scope and single log
	require.Equal(t, 1, logs.ResourceLogs().Len())
	require.Equal(t, 0, logs.ResourceLogs().At(0).Resource().Attributes().Len())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().Len())
	require.Equal(t, metadata.ScopeName, logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom":     "datapoint",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())

	// adding another envelope matching the same resource attributes

	envelope = &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom2":    "datapoint2",
		},
	}

	buildLogs(logs, envelope, observedTime)

	// check that another resource was created
	require.Equal(t, 2, logs.ResourceLogs().Len())
	require.Equal(t, 0, logs.ResourceLogs().At(0).Resource().Attributes().Len())
	require.Equal(t, 0, logs.ResourceLogs().At(1).Resource().Attributes().Len())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().Len())
	require.Equal(t, metadata.ScopeName, logs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	require.Equal(t, 1, logs.ResourceLogs().At(1).ScopeLogs().Len())
	require.Equal(t, metadata.ScopeName, logs.ResourceLogs().At(1).ScopeLogs().At(0).Scope().Name())
	require.Equal(t, 1, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom":     "datapoint",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())
	require.Equal(t, 1, logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom2":    "datapoint2",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())
}

func TestBuildMetricsWithResourceAttrs(t *testing.T) {
	metrics := pmetric.NewMetrics()
	observedTime := time.Date(2022, time.July, 14, 9, 30, 0, 0, time.UTC)

	require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(allowResourceAttributes.ID(), false))
	})

	// adding the first item to the metrics

	envelope := &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom":     "datapoint",
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"memory": {
						Unit:  "bytes",
						Value: 17046641.0,
					},
				},
			},
		},
	}

	buildMetrics(metrics, envelope, observedTime)

	// check if the first metric resource was created successfully
	// we await single resource, with single scope and single metric
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, metrics.ResourceMetrics().At(0).Resource().Attributes().AsRaw())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, metadata.ScopeName, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom": "datapoint",
	}, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())

	// adding another envelope matching the same resource attributes

	envelope = &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom2":    "datapoint2",
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"memory": {
						Unit:  "bytes",
						Value: 17046641.0,
					},
				},
			},
		},
	}

	buildMetrics(metrics, envelope, observedTime)

	// check that the first metric resource contains a single scope, but 2 metrics
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, metrics.ResourceMetrics().At(0).Resource().Attributes().AsRaw())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, metadata.ScopeName, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, 2, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom": "datapoint",
	}, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom2": "datapoint2",
	}, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().AsRaw())

	// adding another envelope not matching the resource attributes

	envelope = &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa33",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom3":    "datapoint3",
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"memory": {
						Unit:  "bytes",
						Value: 17046641.0,
					},
				},
			},
		},
	}

	buildMetrics(metrics, envelope, observedTime)

	// check that the new resource was created and exists next to the original one
	require.Equal(t, 2, metrics.ResourceMetrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa33",
	}, metrics.ResourceMetrics().At(1).Resource().Attributes().AsRaw())
	require.Equal(t, 1, metrics.ResourceMetrics().At(1).ScopeMetrics().Len())
	require.Equal(t, metadata.ScopeName, metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom3": "datapoint3",
	}, metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())
}

func TestBuildMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	observedTime := time.Date(2022, time.July, 14, 9, 30, 0, 0, time.UTC)

	// adding the first item to the metrics

	envelope := &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom":     "datapoint",
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"memory": {
						Unit:  "bytes",
						Value: 17046641.0,
					},
				},
			},
		},
	}

	buildMetrics(metrics, envelope, observedTime)

	// check if the first metric resource was created successfully
	// we await single resource, with single scope and single metric
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, 0, metrics.ResourceMetrics().At(0).Resource().Attributes().Len())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, metadata.ScopeName, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom":     "datapoint",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())

	// adding another envelope matching the resource attributes

	envelope = &loggregator_v2.Envelope{
		Timestamp: observedTime.Unix(),
		SourceId:  "uaa",
		Tags: map[string]string{
			"origin":     "gorouter",
			"deployment": "cf",
			"job":        "router",
			"index":      "bc276108-8282-48a5-bae7-c009c4392246",
			"ip":         "10.244.0.34",
			"custom2":    "datapoint2",
		},
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"memory": {
						Unit:  "bytes",
						Value: 17046641.0,
					},
				},
			},
		},
	}

	buildMetrics(metrics, envelope, observedTime)

	// check that another resource was created
	require.Equal(t, 2, metrics.ResourceMetrics().Len())
	require.Equal(t, 0, metrics.ResourceMetrics().At(0).Resource().Attributes().Len())
	require.Equal(t, 0, metrics.ResourceMetrics().At(1).Resource().Attributes().Len())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, metadata.ScopeName, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(1).ScopeMetrics().Len())
	require.Equal(t, metadata.ScopeName, metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom":     "datapoint",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())
	require.Equal(t, 1, metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, map[string]any{
		"org.cloudfoundry.custom2":    "datapoint2",
		"org.cloudfoundry.origin":     "gorouter",
		"org.cloudfoundry.deployment": "cf",
		"org.cloudfoundry.job":        "router",
		"org.cloudfoundry.index":      "bc276108-8282-48a5-bae7-c009c4392246",
		"org.cloudfoundry.ip":         "10.244.0.34",
		"org.cloudfoundry.source_id":  "uaa",
	}, metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().AsRaw())
}
