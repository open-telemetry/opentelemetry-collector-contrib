// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

func TestSerializeMetric(t *testing.T) {
	logger := zap.NewNop()
	defaultDims := dimensions.NewNormalizedDimensionList(dimensions.NewDimension("default", "value"))
	staticDims := dimensions.NewNormalizedDimensionList(dimensions.NewDimension("static", "value"))

	t.Run("correctly creates a gauge", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		gaugeDp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		gaugeDp.SetIntValue(3)

		prev := ttlmap.New(1, 1)

		serialized, err := SerializeMetric(logger, "prefix", metric, defaultDims, staticDims, prev)
		assert.NoError(t, err)

		assert.Len(t, serialized, 1)

		assertMetricLineTokensEqual(t, serialized[0], "prefix.metric_name,default=value,static=value gauge,3")
	})

	t.Run("correctly creates a counter from a sum", func(t *testing.T) {
		// more in-depth tests for the different sum serializations are in sum_test.go
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		sum := metric.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		sumDp := sum.DataPoints().AppendEmpty()
		sumDp.SetIntValue(4)

		prev := ttlmap.New(1, 1)

		serialized, err := SerializeMetric(logger, "prefix", metric, defaultDims, staticDims, prev)
		assert.NoError(t, err)

		assert.Len(t, serialized, 1)

		assertMetricLineTokensEqual(t, serialized[0], "prefix.metric_name,default=value,static=value count,delta=4")
	})

	t.Run("correctly creates a summary from histogram", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("metric_name")
		hist := metric.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := hist.DataPoints().AppendEmpty()
		dp.SetMin(1)
		dp.SetMax(3)
		dp.SetSum(6)
		dp.SetCount(3)

		prev := ttlmap.New(1, 1)

		serialized, err := SerializeMetric(logger, "prefix", metric, defaultDims, staticDims, prev)
		assert.NoError(t, err)

		assert.Len(t, serialized, 1)

		assertMetricLineTokensEqual(t, serialized[0], "prefix.metric_name,default=value,static=value gauge,min=1,max=3,sum=6,count=3")
	})

}

func Test_makeCombinedDimensions(t *testing.T) {
	defaultDims := dimensions.NewNormalizedDimensionList(
		dimensions.NewDimension("a", "default"),
		dimensions.NewDimension("b", "default"),
		dimensions.NewDimension("c", "default"),
	)
	attributes := pcommon.NewMap()
	attributes.PutStr("a", "attribute")
	attributes.PutStr("b", "attribute")
	staticDims := dimensions.NewNormalizedDimensionList(
		dimensions.NewDimension("a", "static"),
	)
	expected := dimensions.NewNormalizedDimensionList(
		dimensions.NewDimension("a", "static"),
		dimensions.NewDimension("b", "attribute"),
		dimensions.NewDimension("c", "default"),
	)

	actual := makeCombinedDimensions(defaultDims, attributes, staticDims)

	sortAndStringify :=
		func(dims []dimensions.Dimension) string {
			sort.Slice(dims, func(i, j int) bool {
				return dims[i].Key < dims[j].Key
			})
			tokens := make([]string, len(dims))
			for i, dim := range dims {
				tokens[i] = fmt.Sprintf("%s=%s", dim.Key, dim.Value)
			}
			return strings.Join(tokens, ",")
		}

	assert.Equal(t, actual.Format(sortAndStringify), expected.Format(sortAndStringify))
}

type simplifiedLogRecord struct {
	message    string
	attributes map[string]string
}

func makeSimplifiedLogRecordsFromObservedLogs(observedLogs *observer.ObservedLogs) []simplifiedLogRecord {
	observedLogRecords := make([]simplifiedLogRecord, observedLogs.Len())

	for i, observedLog := range observedLogs.All() {
		contextStringMap := make(map[string]string, len(observedLog.ContextMap()))
		for k, v := range observedLog.ContextMap() {
			contextStringMap[k] = fmt.Sprint(v)
		}
		observedLogRecords[i] = simplifiedLogRecord{
			message:    observedLog.Message,
			attributes: contextStringMap,
		}
	}
	return observedLogRecords
}

// tokenizes Dynatrace metric lines
// this ensures that comparing metric lines can be done without
// taking into account the order of dimensions, which does not matter
// the first string contains the name, and the array contains all other tokens
func tokenizeMetricLine(s string) (string, []string) {
	tokens := strings.Split(s, " ")
	if len(tokens) > 0 {
		nameAndDims := strings.Split(tokens[0], ",")
		if len(nameAndDims) > 0 {
			name := nameAndDims[0]
			result := nameAndDims[1:]
			result = append(result, tokens[1:]...)
			return name, result
		}
	}
	return "", nil
}

// asserts that the metric name as well as the dimensions, value, and timestamps match, ignoring dimension order
func assertMetricLineTokensEqual(t *testing.T, a string, b string) {
	nameA, tokensA := tokenizeMetricLine(a)
	nameB, tokensB := tokenizeMetricLine(b)

	assert.Equal(t, nameA, nameB)
	assert.ElementsMatch(t, tokensA, tokensB)
}
