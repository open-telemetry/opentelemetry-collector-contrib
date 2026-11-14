// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostMetrics(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		hosts                  []string
		expectedHostCount      int
		expectedMetricCount    int
		expectedDatapointCount int
	}{
		{
			name:                   "single host",
			hosts:                  []string{"hostA"},
			expectedHostCount:      1,
			expectedMetricCount:    1,
			expectedDatapointCount: 1,
		},
		{
			name:                   "multiple hosts",
			hosts:                  []string{"hostA", "hostB", "hostC", "hostA", "hostB"},
			expectedHostCount:      3,
			expectedMetricCount:    1,
			expectedDatapointCount: 3,
		},
		{
			name:  "none",
			hosts: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hm := newHostMetrics()
			for _, h := range tc.hosts {
				hm.add(h)
			}

			metrics, count := hm.metrics()
			hm.reset()
			require.Equal(t, tc.expectedHostCount, count)
			if metrics != nil {
				require.Equal(t, tc.expectedMetricCount, metrics.MetricCount())
				require.Equal(t, tc.expectedDatapointCount, metrics.DataPointCount())
				rm := metrics.ResourceMetrics()
				metric := rm.At(0).ScopeMetrics().At(0).Metrics().At(0)
				require.Equal(t, hostInfoMetric, metric.Name())
				for i := range count {
					dp := metric.Gauge().DataPoints().At(i)
					val, ok := dp.Attributes().Get(hostIdentifierAttr)
					require.True(t, ok)
					require.True(t, strings.HasPrefix(val.AsString(), "host"))
					require.Equal(t, int64(1), dp.IntValue())
				}
			}
		})
	}
}
