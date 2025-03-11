// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var testMetadata = map[string]scrape.MetricMetadata{
	"counter_test":    {MetricFamily: "counter_test", Type: model.MetricTypeCounter, Help: "", Unit: ""},
	"counter_test2":   {MetricFamily: "counter_test2", Type: model.MetricTypeCounter, Help: "", Unit: ""},
	"gauge_test":      {MetricFamily: "gauge_test", Type: model.MetricTypeGauge, Help: "", Unit: ""},
	"gauge_test2":     {MetricFamily: "gauge_test2", Type: model.MetricTypeGauge, Help: "", Unit: ""},
	"hist_test":       {MetricFamily: "hist_test", Type: model.MetricTypeHistogram, Help: "", Unit: ""},
	"hist_test2":      {MetricFamily: "hist_test2", Type: model.MetricTypeHistogram, Help: "", Unit: ""},
	"ghist_test":      {MetricFamily: "ghist_test", Type: model.MetricTypeGaugeHistogram, Help: "", Unit: ""},
	"summary_test":    {MetricFamily: "summary_test", Type: model.MetricTypeSummary, Help: "", Unit: ""},
	"summary_test2":   {MetricFamily: "summary_test2", Type: model.MetricTypeSummary, Help: "", Unit: ""},
	"unknown_test":    {MetricFamily: "unknown_test", Type: model.MetricTypeUnknown, Help: "", Unit: ""},
	"poor_name":       {MetricFamily: "poor_name", Type: model.MetricTypeGauge, Help: "", Unit: ""},
	"poor_name_count": {MetricFamily: "poor_name_count", Type: model.MetricTypeCounter, Help: "", Unit: ""},
	"scrape_foo":      {MetricFamily: "scrape_foo", Type: model.MetricTypeCounter, Help: "", Unit: ""},
	"example_process_start_time_seconds": {
		MetricFamily: "example_process_start_time_seconds",
		Type:         model.MetricTypeGauge, Help: "", Unit: "",
	},
	"process_start_time_seconds": {
		MetricFamily: "process_start_time_seconds",
		Type:         model.MetricTypeGauge, Help: "", Unit: "",
	},
	"subprocess_start_time_seconds": {
		MetricFamily: "subprocess_start_time_seconds",
		Type:         model.MetricTypeGauge, Help: "", Unit: "",
	},
}

func TestTimestampFromMs(t *testing.T) {
	assert.Equal(t, pcommon.Timestamp(0), timestampFromMs(0))
	assert.Equal(t, pcommon.NewTimestampFromTime(time.UnixMilli(1662679535432)), timestampFromMs(1662679535432))
}

func TestTimestampFromFloat64(t *testing.T) {
	assert.Equal(t, pcommon.Timestamp(0), timestampFromFloat64(0))
	// Because of float64 conversion, we check only that we are within 100ns error.
	assert.InEpsilon(t, uint64(1662679535040000000), uint64(timestampFromFloat64(1662679535.040)), 100)
}

func TestConvToMetricType(t *testing.T) {
	tests := []struct {
		name          string
		mtype         model.MetricType
		want          pmetric.MetricType
		wantMonotonic bool
	}{
		{
			name:          "model.counter",
			mtype:         model.MetricTypeCounter,
			want:          pmetric.MetricTypeSum,
			wantMonotonic: true,
		},
		{
			name:          "model.gauge",
			mtype:         model.MetricTypeGauge,
			want:          pmetric.MetricTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "model.unknown",
			mtype:         model.MetricTypeUnknown,
			want:          pmetric.MetricTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "model.histogram",
			mtype:         model.MetricTypeHistogram,
			want:          pmetric.MetricTypeHistogram,
			wantMonotonic: true,
		},
		{
			name:          "model.summary",
			mtype:         model.MetricTypeSummary,
			want:          pmetric.MetricTypeSummary,
			wantMonotonic: true,
		},
		{
			name:          "model.metric_type_info",
			mtype:         model.MetricTypeInfo,
			want:          pmetric.MetricTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "model.metric_state_set",
			mtype:         model.MetricTypeStateset,
			want:          pmetric.MetricTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "model.metric_gauge_histogram",
			mtype:         model.MetricTypeGaugeHistogram,
			want:          pmetric.MetricTypeEmpty,
			wantMonotonic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, monotonic := convToMetricType(tt.mtype)
			require.Equal(t, got.String(), tt.want.String())
			require.Equal(t, tt.wantMonotonic, monotonic)
		})
	}
}

func TestGetBoundary(t *testing.T) {
	tests := []struct {
		name      string
		mtype     pmetric.MetricType
		labels    labels.Labels
		wantValue float64
		wantErr   error
	}{
		{
			name:      "cumulative histogram with bucket label",
			mtype:     pmetric.MetricTypeHistogram,
			labels:    labels.FromStrings(model.BucketLabel, "0.256"),
			wantValue: 0.256,
		},
		{
			name:      "gauge histogram with bucket label",
			mtype:     pmetric.MetricTypeHistogram,
			labels:    labels.FromStrings(model.BucketLabel, "11.71"),
			wantValue: 11.71,
		},
		{
			name:    "summary with bucket label",
			mtype:   pmetric.MetricTypeSummary,
			labels:  labels.FromStrings(model.BucketLabel, "11.71"),
			wantErr: errEmptyQuantileLabel,
		},
		{
			name:      "summary with quantile label",
			mtype:     pmetric.MetricTypeSummary,
			labels:    labels.FromStrings(model.QuantileLabel, "92.88"),
			wantValue: 92.88,
		},
		{
			name:    "gauge histogram mismatched with bucket label",
			mtype:   pmetric.MetricTypeSummary,
			labels:  labels.FromStrings(model.BucketLabel, "11.71"),
			wantErr: errEmptyQuantileLabel,
		},
		{
			name:    "other data types without matches",
			mtype:   pmetric.MetricTypeGauge,
			labels:  labels.FromStrings(model.BucketLabel, "11.71"),
			wantErr: errNoBoundaryLabel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := getBoundary(tt.mtype, tt.labels)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}
