// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serialization

import (
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func Test_serializeSummaryPoint(t *testing.T) {
	summary := pmetric.NewSummaryDataPoint()
	fillTestValueAtQuantileSlice(summary.QuantileValues(), []float64{1, 2, 3, 4, 5, 6, 7})
	summary.SetCount(2)
	summary.SetSum(9.5)
	summary.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

	t.Run("delta with prefix and dimension", func(t *testing.T) {
		got, err := serializeSummaryPoint("delta_summary", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), summary)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_summary,key=value gauge,min=1,max=7,sum=9.5,count=2 1626438600000", got)
	})

	t.Run("out of order quantile", func(t *testing.T) {
		summaryOutOfOrderQuantile := pmetric.NewSummaryDataPoint()
		fillTestValueAtQuantileSlice(summaryOutOfOrderQuantile.QuantileValues(), []float64{5, 3, 1, 8, 4, 2, 7, 6})
		summaryOutOfOrderQuantile.SetCount(2)
		summaryOutOfOrderQuantile.SetSum(9.5)
		summaryOutOfOrderQuantile.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeSummaryPoint("delta_out_of_order_quantile", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), summaryOutOfOrderQuantile)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_out_of_order_quantile,key=value gauge,min=1,max=8,sum=9.5,count=2 1626438600000", got)
	})

	t.Run("empty quantile", func(t *testing.T) {
		summaryEmptyQuantile := pmetric.NewSummaryDataPoint()
		summaryEmptyQuantile.SetCount(2)
		summaryEmptyQuantile.SetSum(9.5)
		summaryEmptyQuantile.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeSummaryPoint("delta_empty_quantile", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), summaryEmptyQuantile)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_empty_quantile,key=value gauge,min=0,max=0,sum=9.5,count=2 1626438600000", got)
	})

	t.Run("mean quantile > max", func(t *testing.T) {
		summaryMeanQuantile := pmetric.NewSummaryDataPoint()
		fillTestValueAtQuantileSlice(summaryMeanQuantile.QuantileValues(), []float64{1, 2, 3, 4, 5, 6, 7})
		summaryMeanQuantile.SetCount(2)
		summaryMeanQuantile.SetSum(25.0)
		summaryMeanQuantile.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeSummaryPoint("delta_mean_quantile", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), summaryMeanQuantile)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_mean_quantile,key=value gauge,min=1,max=12.5,sum=25,count=2 1626438600000", got)
	})

	t.Run("mean quantile < min", func(t *testing.T) {
		summaryMeanQuantile := pmetric.NewSummaryDataPoint()
		fillTestValueAtQuantileSlice(summaryMeanQuantile.QuantileValues(), []float64{10, 11, 12, 13, 14, 15})
		summaryMeanQuantile.SetCount(2)
		summaryMeanQuantile.SetSum(10)
		summaryMeanQuantile.SetTimestamp(pcommon.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeSummaryPoint("delta_mean_quantile", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), summaryMeanQuantile)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.delta_mean_quantile,key=value gauge,min=5,max=15,sum=10,count=2 1626438600000", got)
	})
}

func Test_serializeSummary(t *testing.T) {
	emptyDims := dimensions.NewNormalizedDimensionList()
	metric := pmetric.NewMetric()
	metric.SetDataType(pmetric.MetricDataTypeSummary)
	metric.SetName("metric_name")
	summary := metric.Summary()
	dp := summary.DataPoints().AppendEmpty()
	fillTestValueAtQuantileSlice(dp.QuantileValues(), []float64{1, 2, 3, 4, 5, 6, 7})
	dp.SetCount(3)
	dp.SetSum(8)

	zapCore, observedLogs := observer.New(zap.WarnLevel)
	logger := zap.New(zapCore)

	lines := serializeSummary(logger, "", metric, emptyDims, emptyDims, []string{})

	expectedLines := []string{
		"metric_name gauge,min=1,max=7,sum=8,count=3",
	}

	assert.ElementsMatch(t, lines, expectedLines)

	assert.Empty(t, observedLogs.All())
}

func fillTestValueAtQuantileSlice(tv pmetric.ValueAtQuantileSlice, values []float64) {
	tv.EnsureCapacity(len(values))
	for i := 0; i < len(values); i++ {
		fillTestValueAtQuantile(tv.AppendEmpty(), values[i])
	}
}

func fillTestValueAtQuantile(tv pmetric.ValueAtQuantile, v float64) {
	tv.SetQuantile(v)
	tv.SetValue(v)
}
