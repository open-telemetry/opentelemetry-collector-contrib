// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAssertMetrics_RejectsInvalidMetrics(t *testing.T) {
	tests := map[string]struct {
		mutate  func(pmetric.Metrics)
		wantErr string
	}{
		"duplicate datapoint attributes": {
			mutate: func(m pmetric.Metrics) {
				dps := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Sum().DataPoints()
				dps.At(0).CopyTo(dps.AppendEmpty())
			},
			wantErr: "datapoint at index 2 has duplicate attributes with datapoint at index 0",
		},
		"duplicate metric name": {
			mutate: func(m pmetric.Metrics) {
				ms := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				ms.At(0).CopyTo(ms.AppendEmpty())
			},
			wantErr: `metric "svc.active" at index 2 is a duplicate of metric at index 0`,
		},
		"duplicate scope": {
			mutate: func(m pmetric.Metrics) {
				sms := m.ResourceMetrics().At(0).ScopeMetrics()
				sms.At(0).CopyTo(sms.AppendEmpty())
			},
			wantErr: "at index 1 is a duplicate of scope at index 0",
		},
		"duplicate resource": {
			mutate: func(m pmetric.Metrics) {
				rms := m.ResourceMetrics()
				rms.At(0).CopyTo(rms.AppendEmpty())
			},
			wantErr: "at index 1 is a duplicate of resource at index 0",
		},
		"resource without scopes": {
			mutate: func(m pmetric.Metrics) {
				m.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("service.name", "other")
			},
			wantErr: "at index 1 has no scope metrics",
		},
		"scope without metrics": {
			mutate: func(m pmetric.Metrics) {
				m.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Scope().SetName("empty")
			},
			wantErr: `scope "empty" at index 1 has no metrics`,
		},
		"metric without datapoints": {
			mutate: func(m pmetric.Metrics) {
				metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				metric.SetName("svc.empty")
				metric.SetEmptyGauge()
			},
			wantErr: `metric "svc.empty" has no datapoints`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m := buildSampleMetrics()
			tt.mutate(m)

			path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
			require.NoError(t, WriteAssertionFile(t, path, m))

			err := AssertMetrics(path, m)
			require.ErrorContains(t, err, "invalid actual metrics: ")
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestAssertMetrics_ReportsInvalidMetricsBeforeReadingFile(t *testing.T) {
	m := buildSampleMetrics()
	rms := m.ResourceMetrics()
	rms.At(0).CopyTo(rms.AppendEmpty())

	err := AssertMetrics(filepath.Join(t.TempDir(), "missing.assert.yaml"), m)
	require.ErrorContains(t, err, "invalid actual metrics: ")
}
