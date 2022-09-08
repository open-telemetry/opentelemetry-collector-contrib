// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	startTs                 = int64(1555366610000)
	interval                = int64(15 * 1000)
	defaultBuilderStartTime = float64(1.0)
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
	"badprocess_start_time_seconds": {Metric: "badprocess_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
}

func TestConvToMetricType(t *testing.T) {
	tests := []struct {
		name          string
		mtype         textparse.MetricType
		want          pmetric.MetricDataType
		wantMonotonic bool
	}{
		{
			name:          "textparse.counter",
			mtype:         textparse.MetricTypeCounter,
			want:          pmetric.MetricDataTypeSum,
			wantMonotonic: true,
		},
		{
			name:          "textparse.gauge",
			mtype:         textparse.MetricTypeGauge,
			want:          pmetric.MetricDataTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "textparse.unknown",
			mtype:         textparse.MetricTypeUnknown,
			want:          pmetric.MetricDataTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "textparse.histogram",
			mtype:         textparse.MetricTypeHistogram,
			want:          pmetric.MetricDataTypeHistogram,
			wantMonotonic: true,
		},
		{
			name:          "textparse.summary",
			mtype:         textparse.MetricTypeSummary,
			want:          pmetric.MetricDataTypeSummary,
			wantMonotonic: true,
		},
		{
			name:          "textparse.metric_type_info",
			mtype:         textparse.MetricTypeInfo,
			want:          pmetric.MetricDataTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "textparse.metric_state_set",
			mtype:         textparse.MetricTypeStateset,
			want:          pmetric.MetricDataTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "textparse.metric_gauge_hostogram",
			mtype:         textparse.MetricTypeGaugeHistogram,
			want:          pmetric.MetricDataTypeNone,
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
		mtype     pmetric.MetricDataType
		labels    labels.Labels
		wantValue float64
		wantErr   error
	}{
		{
			name:      "cumulative histogram with bucket label",
			mtype:     pmetric.MetricDataTypeHistogram,
			labels:    labels.FromStrings(model.BucketLabel, "0.256"),
			wantValue: 0.256,
		},
		{
			name:      "gauge histogram with bucket label",
			mtype:     pmetric.MetricDataTypeHistogram,
			labels:    labels.FromStrings(model.BucketLabel, "11.71"),
			wantValue: 11.71,
		},
		{
			name:    "summary with bucket label",
			mtype:   pmetric.MetricDataTypeSummary,
			labels:  labels.FromStrings(model.BucketLabel, "11.71"),
			wantErr: errEmptyQuantileLabel,
		},
		{
			name:      "summary with quantile label",
			mtype:     pmetric.MetricDataTypeSummary,
			labels:    labels.FromStrings(model.QuantileLabel, "92.88"),
			wantValue: 92.88,
		},
		{
			name:    "gauge histogram mismatched with bucket label",
			mtype:   pmetric.MetricDataTypeSummary,
			labels:  labels.FromStrings(model.BucketLabel, "11.71"),
			wantErr: errEmptyQuantileLabel,
		},
		{
			name:    "other data types without matches",
			mtype:   pmetric.MetricDataTypeGauge,
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
