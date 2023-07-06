// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var testMetadata = map[string]scrape.MetricMetadata{
	"counter_test":    {Metric: "counter_test", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"counter_test2":   {Metric: "counter_test2", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"gauge_test":      {Metric: "gauge_test", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"gauge_test2":     {Metric: "gauge_test2", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"hist_test":       {Metric: "hist_test", Type: textparse.MetricTypeHistogram, Help: "", Unit: ""},
	"hist_test2":      {Metric: "hist_test2", Type: textparse.MetricTypeHistogram, Help: "", Unit: ""},
	"ghist_test":      {Metric: "ghist_test", Type: textparse.MetricTypeGaugeHistogram, Help: "", Unit: ""},
	"summary_test":    {Metric: "summary_test", Type: textparse.MetricTypeSummary, Help: "", Unit: ""},
	"summary_test2":   {Metric: "summary_test2", Type: textparse.MetricTypeSummary, Help: "", Unit: ""},
	"unknown_test":    {Metric: "unknown_test", Type: textparse.MetricTypeUnknown, Help: "", Unit: ""},
	"poor_name":       {Metric: "poor_name", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"poor_name_count": {Metric: "poor_name_count", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"scrape_foo":      {Metric: "scrape_foo", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"example_process_start_time_seconds": {Metric: "example_process_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"process_start_time_seconds": {Metric: "process_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"subprocess_start_time_seconds": {Metric: "subprocess_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
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
		mtype         textparse.MetricType
		want          pmetric.MetricType
		wantMonotonic bool
	}{
		{
			name:          "textparse.counter",
			mtype:         textparse.MetricTypeCounter,
			want:          pmetric.MetricTypeSum,
			wantMonotonic: true,
		},
		{
			name:          "textparse.gauge",
			mtype:         textparse.MetricTypeGauge,
			want:          pmetric.MetricTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "textparse.unknown",
			mtype:         textparse.MetricTypeUnknown,
			want:          pmetric.MetricTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "textparse.histogram",
			mtype:         textparse.MetricTypeHistogram,
			want:          pmetric.MetricTypeHistogram,
			wantMonotonic: true,
		},
		{
			name:          "textparse.summary",
			mtype:         textparse.MetricTypeSummary,
			want:          pmetric.MetricTypeSummary,
			wantMonotonic: true,
		},
		{
			name:          "textparse.metric_type_info",
			mtype:         textparse.MetricTypeInfo,
			want:          pmetric.MetricTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "textparse.metric_state_set",
			mtype:         textparse.MetricTypeStateset,
			want:          pmetric.MetricTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "textparse.metric_gauge_hostogram",
			mtype:         textparse.MetricTypeGaugeHistogram,
			want:          pmetric.MetricTypeEmpty,
			wantMonotonic: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, monotonic := convToMetricType(tt.mtype)
			require.Equal(t, got.String(), tt.want.String())
			require.Equal(t, monotonic, tt.wantMonotonic)
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
			name:      "summary without quantile label",
			mtype:     pmetric.MetricTypeSummary,
			labels:    labels.EmptyLabels(),
			wantValue: 0,
		},
		{
			name:      "summary with quantile label",
			mtype:     pmetric.MetricTypeSummary,
			labels:    labels.FromStrings(model.QuantileLabel, "92.88"),
			wantValue: 92.88,
		},
		{
			name:    "other data types without matches",
			mtype:   pmetric.MetricTypeGauge,
			labels:  labels.FromStrings(model.BucketLabel, "11.71"),
			wantErr: errNoBoundaryLabel,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			value, err := getBoundary(tt.mtype, tt.labels)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, value, tt.wantValue)
		})
	}
}
