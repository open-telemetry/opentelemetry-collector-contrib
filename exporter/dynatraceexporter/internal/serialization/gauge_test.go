// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization

import (
	"math"
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func Test_serializeGaugePoint(t *testing.T) {
	t.Run("float with prefix and dimension", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetDoubleValue(5.5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeGaugePoint("dbl_gauge", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), dp)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.dbl_gauge,key=value gauge,5.5 1626438600000", got)
	})

	t.Run("int with prefix and dimension", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)
		dp.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeGaugePoint("int_gauge", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), dp)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_gauge,key=value gauge,5 1626438600000", got)
	})

	t.Run("without timestamp", func(t *testing.T) {
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(5)

		got, err := serializeGaugePoint("int_gauge", "prefix", dimensions.NewNormalizedDimensionList(), dp)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_gauge gauge,5", got)
	})
}

func Test_serializeGauge(t *testing.T) {
	type args struct {
		prefix            string
		metricName        string
		intValues         []int64
		floatValues       []float64
		defaultDimensions dimensions.NormalizedDimensionList
		staticDimensions  dimensions.NormalizedDimensionList
	}

	tests := []struct {
		name     string
		args     args
		want     []string
		wantLogs []simplifiedLogRecord
	}{
		{
			name: "no data points",
			args: args{
				metricName: "name",
				intValues:  []int64{},
			},
			want:     []string{},
			wantLogs: []simplifiedLogRecord{},
		},
		{
			name: "basic int gauge",
			args: args{
				prefix:     "prefix",
				metricName: "name",
				intValues:  []int64{1, 2, 3},
			},
			want: []string{
				"prefix.name gauge,1",
				"prefix.name gauge,2",
				"prefix.name gauge,3",
			},
			wantLogs: []simplifiedLogRecord{},
		},
		{
			name: "invalid name",
			args: args{
				metricName: ".",
				intValues:  []int64{3},
			},
			want: []string{},
			wantLogs: []simplifiedLogRecord{
				{
					message: "Error serializing gauge data point",
					attributes: map[string]string{
						"name":       ".",
						"value-type": "Int",
						"error":      "first key section is empty (.)",
					},
				},
			},
		},
		{
			name: "invalid double values",
			args: args{
				metricName: "metric_name",
				floatValues: []float64{
					math.Inf(-1),
					math.Inf(1),
					math.NaN(),
				},
			},
			want: []string{},
			wantLogs: []simplifiedLogRecord{
				{
					message: "Error serializing gauge data point",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Double",
						"error":      "value is infinite",
					},
				},
				{
					message: "Error serializing gauge data point",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Double",
						"error":      "value is infinite",
					},
				},
				{
					message: "Error serializing gauge data point",
					attributes: map[string]string{
						"name":       "metric_name",
						"value-type": "Double",
						"error":      "value is NaN.",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zapCore, observedLogs := observer.New(zap.WarnLevel)
			logger := zap.New(zapCore)

			metric := pmetric.NewMetric()
			metric.SetName(tt.args.metricName)
			dataPoints := metric.SetEmptyGauge().DataPoints()

			if tt.args.intValues != nil {
				if tt.args.floatValues != nil {
					t.Fatal("both int and float values set")
				}
				for _, intVal := range tt.args.intValues {
					dataPoints.AppendEmpty().SetIntValue(intVal)
				}
			}
			if tt.args.floatValues != nil {
				for _, floatVal := range tt.args.floatValues {
					dataPoints.AppendEmpty().SetDoubleValue(floatVal)
				}
			}

			actual := serializeGauge(logger, tt.args.prefix, metric, tt.args.defaultDimensions, tt.args.staticDimensions, []string{})

			assert.ElementsMatch(t, actual, tt.want)

			// check that logs contain the expected messages.
			if tt.wantLogs != nil {
				observedLogRecords := makeSimplifiedLogRecordsFromObservedLogs(observedLogs)
				assert.ElementsMatch(t, observedLogRecords, tt.wantLogs)
			}
		})
	}
}
