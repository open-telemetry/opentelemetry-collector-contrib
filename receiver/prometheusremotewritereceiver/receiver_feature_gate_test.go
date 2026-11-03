// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"testing"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
)

func setDisableJobSplitFeatureGate(t *testing.T, enabled bool) {
	t.Helper()
	t.Cleanup(func() {
		assert.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverPrometheusRemoteWriteDisableJobSplitFeatureGate.ID(), false))
	})
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ReceiverPrometheusRemoteWriteDisableJobSplitFeatureGate.ID(), enabled))
}

func TestParseJobAndInstanceFeatureGate(t *testing.T) {
	tests := []struct {
		name     string
		gate     bool
		job      string
		instance string
		expected map[string]any
	}{
		{
			name:     "gate=false splits namespace/service",
			gate:     false,
			job:      "namespace/service",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
				"service.namespace":   "namespace",
				"service.name":        "service",
			},
		},
		{
			name:     "gate=true preserves namespace/service in service.name",
			gate:     true,
			job:      "namespace/service",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
				"service.name":        "namespace/service",
			},
		},
		{
			name:     "gate=false my-job",
			gate:     false,
			job:      "my-job",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
				"service.name":        "my-job",
			},
		},
		{
			name:     "gate=true my-job",
			gate:     true,
			job:      "my-job",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
				"service.name":        "my-job",
			},
		},
		{
			name:     "gate=false multiple slashes a/b/c",
			gate:     false,
			job:      "a/b/c",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
				"service.name":        "a/b/c",
			},
		},
		{
			name:     "gate=true multiple slashes a/b/c",
			gate:     true,
			job:      "a/b/c",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
				"service.name":        "a/b/c",
			},
		},
		{
			name:     "gate=false empty job",
			gate:     false,
			job:      "",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
			},
		},
		{
			name:     "gate=true empty job",
			gate:     true,
			job:      "",
			instance: "instance-1",
			expected: map[string]any{
				"service.instance.id": "instance-1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setDisableJobSplitFeatureGate(t, tc.gate)

			dest := pcommon.NewMap()
			parseJobAndInstance(dest, tc.job, tc.instance)
			assert.Equal(t, tc.expected, dest.AsRaw())
		})
	}
}

func TestTranslateV2DisableJobSplitFeatureGate(t *testing.T) {
	setDisableJobSplitFeatureGate(t, true)

	t.Run("without target_info keeps namespace/service in service.name", func(t *testing.T) {
		prwReceiver := setupMetricsReceiver(t)
		req := &writev2.Request{
			Symbols: []string{
				"",
				"__name__", "test_metric", // 1, 2
				"instance", "host-1", // 3, 4
				"job", "namespace/service", // 5, 6
			},
			Timeseries: []writev2.TimeSeries{
				{
					Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
					LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
					Samples:    []writev2.Sample{{Value: 1, Timestamp: 1000}},
				},
			},
		}

		m, _, err := prwReceiver.translateV2(t.Context(), req)
		require.NoError(t, err)
		require.Equal(t, 1, m.ResourceMetrics().Len())

		resAttrs := m.ResourceMetrics().At(0).Resource().Attributes()
		assert.Equal(t, map[string]any{
			"service.name":        "namespace/service",
			"service.instance.id": "host-1",
		}, resAttrs.AsRaw())
		_, hasNamespace := resAttrs.Get("service.namespace")
		assert.False(t, hasNamespace)
	})

	t.Run("target_info service.name and service.namespace take precedence", func(t *testing.T) {
		prwReceiver := setupMetricsReceiver(t)
		req := &writev2.Request{
			Symbols: []string{
				"",
				"__name__", "target_info", "test_metric", // 1, 2, 3
				"instance", "host-1", // 4, 5
				"job", "namespace/service", // 6, 7
				"service.name", "explicit-service", // 8, 9
				"service.namespace", "explicit-namespace", // 10, 11
			},
			Timeseries: []writev2.TimeSeries{
				{
					Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
					LabelsRefs: []uint32{1, 2, 4, 5, 6, 7, 8, 9, 10, 11},
					Samples:    []writev2.Sample{{Value: 1, Timestamp: 1000}},
				},
				{
					Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
					LabelsRefs: []uint32{1, 3, 4, 5, 6, 7},
					Samples:    []writev2.Sample{{Value: 42, Timestamp: 1000}},
				},
			},
		}

		m, _, err := prwReceiver.translateV2(t.Context(), req)
		require.NoError(t, err)
		require.Equal(t, 1, m.ResourceMetrics().Len())

		resAttrs := m.ResourceMetrics().At(0).Resource().Attributes()
		assert.Equal(t, map[string]any{
			"service.name":        "explicit-service",
			"service.namespace":   "explicit-namespace",
			"service.instance.id": "host-1",
		}, resAttrs.AsRaw())
	})
}
