// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestDecodeEvent(t *testing.T) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	jsonData, err := os.ReadFile(filepath.Join("testdata", "event.json"))
	require.NoError(t, err)

	var records []collectDRecord
	err = json.Unmarshal(jsonData, &records)
	require.NoError(t, err)

	for _, cdr := range records {
		err := cdr.appendToMetrics(zap.NewNop(), scopeMetrics, map[string]string{})
		assert.NoError(t, err)
		assert.Equal(t, 0, metrics.MetricCount())
	}
}

func TestDecodeMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	scopeMemtrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	jsonData, err := os.ReadFile(filepath.Join("testdata", "collectd.json"))
	require.NoError(t, err)

	var records []collectDRecord
	err = json.Unmarshal(jsonData, &records)
	require.NoError(t, err)

	for _, cdr := range records {
		err = cdr.appendToMetrics(zap.NewNop(), scopeMemtrics, map[string]string{})
		assert.NoError(t, err)
	}
	assert.Equal(t, 10, metrics.MetricCount())

	assertMetricsEqual(t, metrics)
}

func createPtrFloat64(v float64) *float64 {
	return &v
}

func TestStartTimestamp(t *testing.T) {
	tests := []struct {
		name                 string
		record               collectDRecord
		metricDescriptorType string
		wantStartTimestamp   pcommon.Timestamp
	}{
		{
			name: "metric type cumulative distribution",
			record: collectDRecord{
				Time:     createPtrFloat64(10),
				Interval: createPtrFloat64(5),
			},
			metricDescriptorType: "cumulative",
			wantStartTimestamp:   pcommon.NewTimestampFromTime(time.Unix(5, 0)),
		},
		{
			name: "metric type cumulative double",
			record: collectDRecord{
				Time:     createPtrFloat64(10),
				Interval: createPtrFloat64(5),
			},
			metricDescriptorType: "cumulative",
			wantStartTimestamp:   pcommon.NewTimestampFromTime(time.Unix(5, 0)),
		},
		{
			name: "metric type cumulative int64",
			record: collectDRecord{
				Time:     createPtrFloat64(10),
				Interval: createPtrFloat64(5),
			},
			metricDescriptorType: "cumulative",
			wantStartTimestamp:   pcommon.NewTimestampFromTime(time.Unix(5, 0)),
		},
		{
			name: "metric type non-cumulative gauge distribution",
			record: collectDRecord{
				Time:     createPtrFloat64(0),
				Interval: createPtrFloat64(0),
			},
			metricDescriptorType: "gauge",
			wantStartTimestamp:   pcommon.NewTimestampFromTime(time.Unix(0, 0)),
		},
		{
			name: "metric type non-cumulative gauge int64",
			record: collectDRecord{
				Time:     createPtrFloat64(0),
				Interval: createPtrFloat64(0),
			},
			metricDescriptorType: "gauge",
			wantStartTimestamp:   pcommon.NewTimestampFromTime(time.Unix(0, 0)),
		},
		{
			name: "metric type non-cumulativegauge double",
			record: collectDRecord{
				Time:     createPtrFloat64(0),
				Interval: createPtrFloat64(0),
			},
			metricDescriptorType: "gauge",
			wantStartTimestamp:   pcommon.NewTimestampFromTime(time.Unix(0, 0)),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStartTimestamp := tc.record.startTimestamp(tc.metricDescriptorType)
			assert.Equal(t, tc.wantStartTimestamp.AsTime(), gotStartTimestamp.AsTime())
		})
	}
}

func assertMetricsEqual(t *testing.T, actual pmetric.Metrics) {
	goldenPath := filepath.Join("testdata", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(goldenPath)
	require.NoError(t, err)

	err = pmetrictest.CompareMetrics(expectedMetrics, actual, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder())
	require.NoError(t, err)
}
