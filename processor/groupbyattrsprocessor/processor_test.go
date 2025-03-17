// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadatatest"
)

var attrMap = prepareAttributeMap()

func prepareAttributeMap() pcommon.Map {
	am := pcommon.NewMap()
	am.PutStr("xx", "aa")
	am.PutInt("yy", 11)
	return am
}

func prepareResource(attrMap pcommon.Map, selectedKeys []string) pcommon.Resource {
	res := pcommon.NewResource()
	for _, key := range selectedKeys {
		val, found := attrMap.Get(key)
		if found {
			val.CopyTo(res.Attributes().PutEmpty(key))
		}
	}
	return res
}

func filterAttributeMap(attrMap pcommon.Map, selectedKeys []string) pcommon.Map {
	filteredAttrMap := pcommon.NewMap()
	if len(selectedKeys) == 0 {
		return filteredAttrMap
	}

	filteredAttrMap.EnsureCapacity(10)
	for _, key := range selectedKeys {
		val, _ := attrMap.Get(key)
		val.CopyTo(filteredAttrMap.PutEmpty(key))
	}
	return filteredAttrMap
}

func someComplexLogs(withResourceAttrIndex bool, rlCount int, illCount int) plog.Logs {
	logs := plog.NewLogs()

	for i := 0; i < rlCount; i++ {
		rl := logs.ResourceLogs().AppendEmpty()
		if withResourceAttrIndex {
			rl.Resource().Attributes().PutInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < illCount; j++ {
			log := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			log.Attributes().PutStr("commonGroupedAttr", "abc")
			log.Attributes().PutStr("commonNonGroupedAttr", "xyz")
		}
	}

	return logs
}

func someComplexTraces(withResourceAttrIndex bool, rsCount int, ilsCount int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < rsCount; i++ {
		rs := traces.ResourceSpans().AppendEmpty()
		if withResourceAttrIndex {
			rs.Resource().Attributes().PutInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < ilsCount; j++ {
			span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			span.Attributes().PutStr("commonGroupedAttr", "abc")
			span.Attributes().PutStr("commonNonGroupedAttr", "xyz")
		}
	}

	return traces
}

func someComplexMetrics(withResourceAttrIndex bool, rmCount int, ilmCount int, dataPointCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	for i := 0; i < rmCount; i++ {
		rm := metrics.ResourceMetrics().AppendEmpty()
		if withResourceAttrIndex {
			rm.Resource().Attributes().PutInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < ilmCount; j++ {
			metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			dps := metric.SetEmptyGauge().DataPoints()

			for k := 0; k < dataPointCount; k++ {
				dataPoint := dps.AppendEmpty()
				dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				dataPoint.SetIntValue(int64(k))
				dataPoint.Attributes().PutStr("commonGroupedAttr", "abc")
				dataPoint.Attributes().PutStr("commonNonGroupedAttr", "xyz")
			}
		}
	}

	return metrics
}

func someComplexHistogramMetrics(withResourceAttrIndex bool, rmCount int, ilmCount int, dataPointCount int, histogramSize int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	for i := 0; i < rmCount; i++ {
		rm := metrics.ResourceMetrics().AppendEmpty()
		if withResourceAttrIndex {
			rm.Resource().Attributes().PutInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < ilmCount; j++ {
			metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

			for k := 0; k < dataPointCount; k++ {
				dataPoint := metric.Histogram().DataPoints().AppendEmpty()
				dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				buckets := randUIntArr(histogramSize)
				sort.Slice(buckets, func(i, j int) bool { return buckets[i] < buckets[j] })
				dataPoint.BucketCounts().FromRaw(buckets)
				dataPoint.ExplicitBounds().FromRaw(randFloat64Arr(histogramSize))
				dataPoint.SetCount(sum(buckets))
				dataPoint.Attributes().PutStr("commonGroupedAttr", "abc")
				dataPoint.Attributes().PutStr("commonNonGroupedAttr", "xyz")
			}
		}
	}

	return metrics
}

func randUIntArr(size int) []uint64 {
	arr := make([]uint64, size)
	for i := 0; i < size; i++ {
		arr[i] = rand.Uint64()
	}
	return arr
}

func sum(arr []uint64) uint64 {
	res := uint64(0)
	for _, v := range arr {
		res += v
	}
	return res
}

func randFloat64Arr(size int) []float64 {
	arr := make([]float64, size)
	for i := 0; i < size; i++ {
		arr[i] = rand.Float64()
	}
	return arr
}

func assertResourceContainsAttributes(t *testing.T, resource pcommon.Resource, attributeMap pcommon.Map) {
	for k, v := range attributeMap.All() {
		rv, found := resource.Attributes().Get(k)
		assert.True(t, found)
		assert.Equal(t, v, rv)
	}
}

// The "complex" use case has following input data:
//   - Resource[Spans|Logs|Metrics] #1
//     Attributes: resourceAttrIndex => <resource_no> (when `withResourceAttrIndex` set to true)
//   - InstrumentationLibrary[Spans|Logs|Metrics] #1
//   - [Span|Log] foo-1-1
//     Attributes: commonGroupedAttr => abc, commonNonGroupedAttr => xyz
//   - Metric foo-1-1
//   - DataPoint #1
//     IntValue: 1
//     Attributes: commonGroupedAttr => abc, commonNonGroupedAttr => xyz
//   - InstrumentationLibrary[Spans|Logs|Metrics] #M
//     ...
//     ...
//   - Resource[Spans|Logs|Metrics] #N
//     ...
func TestComplexAttributeGrouping(t *testing.T) {
	tests := []struct {
		name                              string
		groupByKeys                       []string
		withResourceAttrIndex             bool
		shouldMoveCommonGroupedAttr       bool
		inputResourceCount                int
		inputInstrumentationLibraryCount  int
		outputResourceCount               int
		outputInstrumentationLibraryCount int // Per each Resource
		outputTotalRecordsCount           int // Per each Instrumentation Library
	}{
		{
			name:                             "With not unique Resource-level attributes",
			groupByKeys:                      []string{"commonGroupedAttr"},
			withResourceAttrIndex:            false,
			shouldMoveCommonGroupedAttr:      true,
			inputResourceCount:               4,
			inputInstrumentationLibraryCount: 4,
			// All resources and instrumentation libraries are matching and can be joined together
			outputResourceCount:               1,
			outputInstrumentationLibraryCount: 1,
			outputTotalRecordsCount:           16,
		},
		{
			name:                             "With unique Resource-level attributes",
			groupByKeys:                      []string{"commonGroupedAttr"},
			withResourceAttrIndex:            true,
			shouldMoveCommonGroupedAttr:      true,
			inputResourceCount:               4,
			inputInstrumentationLibraryCount: 4,
			// Since each resource has a unique attribute value, so they cannot be joined together into one
			outputResourceCount:               4,
			outputInstrumentationLibraryCount: 1,
			outputTotalRecordsCount:           16,
		},
		{
			name:                             "Compaction by using empty group by keys",
			groupByKeys:                      []string{},
			withResourceAttrIndex:            false,
			shouldMoveCommonGroupedAttr:      false,
			inputResourceCount:               4,
			inputInstrumentationLibraryCount: 4,
			// This compacts all resources into one actually
			outputResourceCount:               1,
			outputInstrumentationLibraryCount: 1,
			outputTotalRecordsCount:           16,
		},
		{
			name:                             "Compaction by using empty group by keys and grouping resources",
			groupByKeys:                      []string{},
			withResourceAttrIndex:            true,
			shouldMoveCommonGroupedAttr:      false,
			inputResourceCount:               4,
			inputInstrumentationLibraryCount: 4,
			// This does not change the number of resources since they have unique attribute sets
			outputResourceCount:               4,
			outputInstrumentationLibraryCount: 1,
			outputTotalRecordsCount:           16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputLogs := someComplexLogs(tt.withResourceAttrIndex, tt.inputResourceCount, tt.inputInstrumentationLibraryCount)
			inputTraces := someComplexTraces(tt.withResourceAttrIndex, tt.inputResourceCount, tt.inputInstrumentationLibraryCount)
			inputMetrics := someComplexMetrics(tt.withResourceAttrIndex, tt.inputResourceCount, tt.inputInstrumentationLibraryCount, 2)
			inputHistogramMetrics := someComplexHistogramMetrics(tt.withResourceAttrIndex, tt.inputResourceCount, tt.inputInstrumentationLibraryCount, 2, 2)

			tel := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })

			gap, err := createGroupByAttrsProcessor(metadatatest.NewSettings(tel), tt.groupByKeys)
			require.NoError(t, err)

			processedLogs, err := gap.processLogs(context.Background(), inputLogs)
			assert.NoError(t, err)

			processedSpans, err := gap.processTraces(context.Background(), inputTraces)
			assert.NoError(t, err)

			processedMetrics, err := gap.processMetrics(context.Background(), inputMetrics)
			assert.NoError(t, err)

			processedHistogramMetrics, err := gap.processMetrics(context.Background(), inputHistogramMetrics)
			assert.NoError(t, err)

			// Following are record-level attributes that should be preserved after processing
			outputRecordAttrs := pcommon.NewMap()
			outputResourceAttrs := pcommon.NewMap()
			if tt.shouldMoveCommonGroupedAttr {
				// This was present at record level and should be found on Resource level after the processor
				outputResourceAttrs.PutStr("commonGroupedAttr", "abc")
			} else {
				outputRecordAttrs.PutStr("commonGroupedAttr", "abc")
			}
			outputRecordAttrs.PutStr("commonNonGroupedAttr", "xyz")

			rls := processedLogs.ResourceLogs()
			assert.Equal(t, tt.outputResourceCount, rls.Len())
			assert.Equal(t, tt.outputTotalRecordsCount, processedLogs.LogRecordCount())
			for i := 0; i < rls.Len(); i++ {
				rl := rls.At(i)
				assert.Equal(t, tt.outputInstrumentationLibraryCount, rl.ScopeLogs().Len())

				assertResourceContainsAttributes(t, rl.Resource(), outputResourceAttrs)

				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					logs := rl.ScopeLogs().At(j).LogRecords()
					for k := 0; k < logs.Len(); k++ {
						assert.EqualValues(t, outputRecordAttrs, logs.At(k).Attributes())
					}
				}
			}

			rss := processedSpans.ResourceSpans()
			assert.Equal(t, tt.outputResourceCount, rss.Len())
			assert.Equal(t, tt.outputTotalRecordsCount, processedSpans.SpanCount())
			for i := 0; i < rss.Len(); i++ {
				rs := rss.At(i)
				assert.Equal(t, tt.outputInstrumentationLibraryCount, rs.ScopeSpans().Len())

				assertResourceContainsAttributes(t, rs.Resource(), outputResourceAttrs)

				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					spans := rs.ScopeSpans().At(j).Spans()
					for k := 0; k < spans.Len(); k++ {
						assert.EqualValues(t, outputRecordAttrs, spans.At(k).Attributes())
					}
				}
			}

			rms := processedMetrics.ResourceMetrics()
			assert.Equal(t, tt.outputResourceCount, rms.Len())
			assert.Equal(t, tt.outputTotalRecordsCount, processedMetrics.MetricCount())
			for i := 0; i < rms.Len(); i++ {
				rm := rms.At(i)
				assert.Equal(t, tt.outputInstrumentationLibraryCount, rm.ScopeMetrics().Len())

				assertResourceContainsAttributes(t, rm.Resource(), outputResourceAttrs)

				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					metrics := rm.ScopeMetrics().At(j).Metrics()
					for k := 0; k < metrics.Len(); k++ {
						metric := metrics.At(k)
						for l := 0; l < metric.Gauge().DataPoints().Len(); l++ {
							assert.EqualValues(t, outputRecordAttrs, metric.Gauge().DataPoints().At(l).Attributes())
						}
					}
				}
			}

			rmhs := processedHistogramMetrics.ResourceMetrics()
			assert.Equal(t, tt.outputResourceCount, rmhs.Len())
			assert.Equal(t, tt.outputTotalRecordsCount, processedHistogramMetrics.MetricCount())
			for i := 0; i < rmhs.Len(); i++ {
				rm := rmhs.At(i)
				assert.Equal(t, tt.outputInstrumentationLibraryCount, rm.ScopeMetrics().Len())

				assertResourceContainsAttributes(t, rm.Resource(), outputResourceAttrs)

				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					metrics := rm.ScopeMetrics().At(j).Metrics()
					for k := 0; k < metrics.Len(); k++ {
						metric := metrics.At(k)
						assert.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Histogram().AggregationTemporality())
						for l := 0; l < metric.Histogram().DataPoints().Len(); l++ {
							assert.EqualValues(t, outputRecordAttrs, metric.Histogram().DataPoints().At(l).Attributes())
						}
					}
				}
			}
			if tt.shouldMoveCommonGroupedAttr {
				metadatatest.AssertEqualProcessorGroupbyattrsNumGroupedLogs(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: int64(tt.outputTotalRecordsCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsNumGroupedMetrics(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: 4 * int64(tt.outputTotalRecordsCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsNumGroupedSpans(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: int64(tt.outputTotalRecordsCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsLogGroups(t, tel, []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Max:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Sum:          int64(tt.outputResourceCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsMetricGroups(t, tel, []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        2,
						Min:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Max:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Sum:          2 * int64(tt.outputResourceCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsSpanGroups(t, tel, []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Max:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Sum:          int64(tt.outputResourceCount),
					},
				}, metricdatatest.IgnoreTimestamp())
			} else {
				metadatatest.AssertEqualProcessorGroupbyattrsNumNonGroupedLogs(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: int64(tt.outputTotalRecordsCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsNumNonGroupedMetrics(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: 4 * int64(tt.outputTotalRecordsCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsNumNonGroupedSpans(t, tel, []metricdata.DataPoint[int64]{
					{
						Value: int64(tt.outputTotalRecordsCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsLogGroups(t, tel, []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Max:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Sum:          int64(tt.outputResourceCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsMetricGroups(t, tel, []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        2,
						Min:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Max:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Sum:          2 * int64(tt.outputResourceCount),
					},
				}, metricdatatest.IgnoreTimestamp())
				metadatatest.AssertEqualProcessorGroupbyattrsSpanGroups(t, tel, []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Max:          metricdata.NewExtrema(int64(tt.outputResourceCount)),
						Sum:          int64(tt.outputResourceCount),
					},
				}, metricdatatest.IgnoreTimestamp())
			}
		})
	}
}

func TestAttributeGrouping(t *testing.T) {
	tests := []struct {
		name           string
		groupByKeys    []string
		nonGroupedKeys []string
		count          int
	}{
		{
			name:           "Two groupByKeys",
			groupByKeys:    []string{"xx", "yy"},
			nonGroupedKeys: []string{},
			count:          4,
		},
		{
			name:           "One groupByKey",
			groupByKeys:    []string{"xx"},
			nonGroupedKeys: []string{"yy"},
			count:          4,
		},
		{
			name:           "Not matching groupByKeys",
			groupByKeys:    []string{"zz"},
			nonGroupedKeys: []string{"xx", "yy"},
			count:          4,
		},
		{
			name:           "Empty groupByKeys",
			groupByKeys:    []string{},
			nonGroupedKeys: []string{"xx", "yy"},
			count:          4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := someLogs(attrMap, 1, tt.count)
			spans := someSpans(attrMap, 1, tt.count)
			gaugeMetrics := someGaugeMetrics(attrMap, 1, tt.count)
			sumMetrics := someSumMetrics(attrMap, 1, tt.count)
			summaryMetrics := someSummaryMetrics(attrMap, 1, tt.count)
			histogramMetrics := someHistogramMetrics(attrMap, 1, tt.count)
			exponentialHistogramMetrics := someExponentialHistogramMetrics(attrMap, 1, tt.count)

			gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), tt.groupByKeys)
			require.NoError(t, err)

			expectedResource := prepareResource(attrMap, tt.groupByKeys)
			expectedAttributes := filterAttributeMap(attrMap, tt.nonGroupedKeys)

			processedLogs, err := gap.processLogs(context.Background(), logs)
			assert.NoError(t, err)

			processedSpans, err := gap.processTraces(context.Background(), spans)
			assert.NoError(t, err)

			processedGaugeMetrics, err := gap.processMetrics(context.Background(), gaugeMetrics)
			assert.NoError(t, err)

			processedSumMetrics, err := gap.processMetrics(context.Background(), sumMetrics)
			assert.NoError(t, err)

			processedSummaryMetrics, err := gap.processMetrics(context.Background(), summaryMetrics)
			assert.NoError(t, err)

			processedHistogramMetrics, err := gap.processMetrics(context.Background(), histogramMetrics)
			assert.NoError(t, err)

			processedExponentialHistogramMetrics, err := gap.processMetrics(context.Background(), exponentialHistogramMetrics)
			assert.NoError(t, err)

			assert.Equal(t, 1, processedLogs.ResourceLogs().Len())
			assert.Equal(t, 1, processedSpans.ResourceSpans().Len())
			assert.Equal(t, 1, processedGaugeMetrics.ResourceMetrics().Len())
			assert.Equal(t, 1, processedSumMetrics.ResourceMetrics().Len())
			assert.Equal(t, 1, processedSummaryMetrics.ResourceMetrics().Len())
			assert.Equal(t, 1, processedHistogramMetrics.ResourceMetrics().Len())
			assert.Equal(t, 1, processedExponentialHistogramMetrics.ResourceMetrics().Len())

			resources := []pcommon.Resource{
				processedLogs.ResourceLogs().At(0).Resource(),
				processedSpans.ResourceSpans().At(0).Resource(),
				processedGaugeMetrics.ResourceMetrics().At(0).Resource(),
				processedSumMetrics.ResourceMetrics().At(0).Resource(),
				processedSummaryMetrics.ResourceMetrics().At(0).Resource(),
				processedHistogramMetrics.ResourceMetrics().At(0).Resource(),
				processedExponentialHistogramMetrics.ResourceMetrics().At(0).Resource(),
			}

			for _, res := range resources {
				assert.Equal(t, expectedResource.Attributes().AsRaw(), res.Attributes().AsRaw())
			}

			ills := processedLogs.ResourceLogs().At(0).ScopeLogs()
			ilss := processedSpans.ResourceSpans().At(0).ScopeSpans()
			ilgms := processedGaugeMetrics.ResourceMetrics().At(0).ScopeMetrics()
			ilsms := processedSumMetrics.ResourceMetrics().At(0).ScopeMetrics()
			ilsyms := processedSummaryMetrics.ResourceMetrics().At(0).ScopeMetrics()
			ilhms := processedHistogramMetrics.ResourceMetrics().At(0).ScopeMetrics()
			ilehms := processedExponentialHistogramMetrics.ResourceMetrics().At(0).ScopeMetrics()

			assert.Equal(t, 1, ills.Len())
			assert.Equal(t, 1, ilss.Len())
			assert.Equal(t, 1, ilgms.Len())
			assert.Equal(t, 1, ilsms.Len())
			assert.Equal(t, 1, ilsyms.Len())
			assert.Equal(t, 1, ilhms.Len())
			assert.Equal(t, 1, ilehms.Len())

			ls := ills.At(0).LogRecords()
			ss := ilss.At(0).Spans()
			gms := ilgms.At(0).Metrics()
			sms := ilsms.At(0).Metrics()
			syms := ilsyms.At(0).Metrics()
			hms := ilhms.At(0).Metrics()
			ehms := ilehms.At(0).Metrics()
			assert.Equal(t, tt.count, ls.Len())
			assert.Equal(t, tt.count, ss.Len())
			assert.Equal(t, tt.count, gms.Len())
			assert.Equal(t, tt.count, sms.Len())
			assert.Equal(t, tt.count, syms.Len())
			assert.Equal(t, tt.count, hms.Len())
			assert.Equal(t, tt.count, ehms.Len())

			for i := 0; i < ls.Len(); i++ {
				log := ls.At(i)
				span := ss.At(i)
				gaugeDataPoint := gms.At(i).Gauge().DataPoints().At(0)
				sumDataPoint := sms.At(i).Sum().DataPoints().At(0)
				summaryDataPoint := syms.At(i).Summary().DataPoints().At(0)
				histogramDataPoint := hms.At(i).Histogram().DataPoints().At(0)
				exponentialHistogramDataPoint := ehms.At(i).ExponentialHistogram().DataPoints().At(0)

				assert.Equal(t, expectedAttributes.AsRaw(), log.Attributes().AsRaw())
				assert.Equal(t, expectedAttributes.AsRaw(), span.Attributes().AsRaw())
				assert.Equal(t, expectedAttributes.AsRaw(), gaugeDataPoint.Attributes().AsRaw())
				assert.Equal(t, expectedAttributes.AsRaw(), sumDataPoint.Attributes().AsRaw())
				assert.Equal(t, expectedAttributes.AsRaw(), summaryDataPoint.Attributes().AsRaw())
				assert.Equal(t, expectedAttributes.AsRaw(), histogramDataPoint.Attributes().AsRaw())
				assert.Equal(t, expectedAttributes.AsRaw(), exponentialHistogramDataPoint.Attributes().AsRaw())
			}
		})
	}
}

func someSpans(attrs pcommon.Map, instrumentationLibraryCount int, spanCount int) ptrace.Traces {
	traces := ptrace.NewTraces()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < spanCount; j++ {
			ils := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
			ils.Scope().SetName(ilName)
			span := ils.Spans().AppendEmpty()
			span.SetName(fmt.Sprint("foo-", j))
			attrs.CopyTo(span.Attributes())
		}
	}
	return traces
}

func someLogs(attrs pcommon.Map, instrumentationLibraryCount int, logCount int) plog.Logs {
	logs := plog.NewLogs()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < logCount; j++ {
			sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
			sl.Scope().SetName(ilName)
			log := sl.LogRecords().AppendEmpty()
			attrs.CopyTo(log.Attributes())
		}
	}
	return logs
}

func someGaugeMetrics(attrs pcommon.Map, instrumentationLibraryCount int, metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("gauge-", j))
			dataPoint := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someSumMetrics(attrs pcommon.Map, instrumentationLibraryCount int, metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("sum-", j))
			dataPoint := metric.SetEmptySum().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someSummaryMetrics(attrs pcommon.Map, instrumentationLibraryCount int, metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("summary-", j))
			dataPoint := metric.SetEmptySummary().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someHistogramMetrics(attrs pcommon.Map, instrumentationLibraryCount int, metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("histogram-", j))
			dataPoint := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someExponentialHistogramMetrics(attrs pcommon.Map, instrumentationLibraryCount int, metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("exponential-histogram-", j))
			dataPoint := metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func TestMetricAdvancedGrouping(t *testing.T) {
	// Input:
	//
	// Resource {host.name="localhost"}
	//   Metric "gauge-1"
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-B",id="eth0"}
	//   Metric "gauge-1" (same metric name!)
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-B",id="eth0"}
	//   Metric "mixed-type" (GAUGE)
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-B",id="eth0"}
	//   Metric "mixed-type" (SUM!!!)
	//     DataPoint {host.name="host-A",id="eth0"}
	//     DataPoint {host.name="host-A",id="eth0"}
	//   Metric "dont-move" (Gauge)
	//     DataPoint {id="eth0"}
	//
	// Group metrics on host.name attribute
	//
	// Expected Result: 3 Resources (see below)
	//
	// Resource {host.name="localhost"}
	//   Metric "dont-move" (Gauge)
	//     DataPoint {id="eth0"}
	//
	// Resource {host.name="host-A"}
	//   Metric "gauge-1"
	//     DataPoint {id="eth0"}
	//     DataPoint {id="eth0"}
	//     DataPoint {id="eth0"}
	//     DataPoint {id="eth0"}
	//   Metric "mixed-type" (GAUGE)
	//     DataPoint {id="eth0"}
	//     DataPoint {id="eth0"}
	//   Metric "mixed-type" (SUM!!!)
	//     DataPoint {id="eth0"}
	//     DataPoint {id="eth0"}
	//
	// Resource {host.name="host-B"}
	//   Metric "gauge-1"
	//     DataPoint {id="eth0"}
	//     DataPoint {id="eth0"}
	//   Metric "mixed-type" (GAUGE)
	//     DataPoint {id="eth0"}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resourceMetrics.Resource().Attributes().PutStr("host.name", "localhost")

	ilm := resourceMetrics.ScopeMetrics().AppendEmpty()

	// gauge-1
	gauge1 := ilm.Metrics().AppendEmpty()
	gauge1.SetName("gauge-1")
	datapoint := gauge1.SetEmptyGauge().DataPoints().AppendEmpty()
	datapoint.Attributes().PutStr("host.name", "host-A")
	datapoint.Attributes().PutStr("id", "eth0")
	datapoint = gauge1.Gauge().DataPoints().AppendEmpty()
	datapoint.Attributes().PutStr("host.name", "host-A")
	datapoint.Attributes().PutStr("id", "eth0")
	datapoint = gauge1.Gauge().DataPoints().AppendEmpty()
	datapoint.Attributes().PutStr("host.name", "host-B")
	datapoint.Attributes().PutStr("id", "eth0")

	// Duplicate the same metric, with same name and same type
	gauge1.CopyTo(ilm.Metrics().AppendEmpty())

	// mixed-type (GAUGE)
	mixedType1 := ilm.Metrics().AppendEmpty()
	gauge1.CopyTo(mixedType1)
	mixedType1.SetName("mixed-type")

	// mixed-type (same name but different TYPE)
	mixedType2 := ilm.Metrics().AppendEmpty()
	mixedType2.SetName("mixed-type")
	datapoint = mixedType2.SetEmptySum().DataPoints().AppendEmpty()
	datapoint.Attributes().PutStr("host.name", "host-A")
	datapoint.Attributes().PutStr("id", "eth0")
	datapoint = mixedType2.Sum().DataPoints().AppendEmpty()
	datapoint.Attributes().PutStr("host.name", "host-A")
	datapoint.Attributes().PutStr("id", "eth0")

	// dontmove (metric that will not move to another resource)
	dontmove := ilm.Metrics().AppendEmpty()
	dontmove.SetName("dont-move")
	datapoint = dontmove.SetEmptyGauge().DataPoints().AppendEmpty()
	datapoint.Attributes().PutStr("id", "eth0")

	// Perform the test
	gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{"host.name"})
	require.NoError(t, err)

	processedMetrics, err := gap.processMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	// We must have 3 resulting resources
	assert.Equal(t, 3, processedMetrics.ResourceMetrics().Len())

	// We must have localhost
	localhost, foundLocalhost := retrieveHostResource(processedMetrics.ResourceMetrics(), "localhost")
	assert.True(t, foundLocalhost)
	assert.Equal(t, 1, localhost.Resource().Attributes().Len())
	assert.Equal(t, 1, localhost.ScopeMetrics().Len())
	assert.Equal(t, 1, localhost.ScopeMetrics().At(0).Metrics().Len())
	localhostMetric := localhost.ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "dont-move", localhostMetric.Name())
	assert.Equal(t, pmetric.MetricTypeGauge, localhostMetric.Type())

	// We must have host-A
	hostA, foundHostA := retrieveHostResource(processedMetrics.ResourceMetrics(), "host-A")
	assert.True(t, foundHostA)
	assert.Equal(t, 1, hostA.Resource().Attributes().Len())
	assert.Equal(t, 1, hostA.ScopeMetrics().Len())
	assert.Equal(t, 3, hostA.ScopeMetrics().At(0).Metrics().Len())
	hostAGauge1, foundHostAGauge1 := retrieveMetric(hostA.ScopeMetrics().At(0).Metrics(), "gauge-1", pmetric.MetricTypeGauge)
	assert.True(t, foundHostAGauge1)
	assert.Equal(t, 4, hostAGauge1.Gauge().DataPoints().Len())
	assert.Equal(t, 1, hostAGauge1.Gauge().DataPoints().At(0).Attributes().Len())
	metricIDAttribute, foundMetricIDAttribute := hostAGauge1.Gauge().DataPoints().At(0).Attributes().Get("id")
	assert.True(t, foundMetricIDAttribute)
	assert.Equal(t, "eth0", metricIDAttribute.AsString())
	hostAMixedGauge, foundHostAMixedGauge := retrieveMetric(hostA.ScopeMetrics().At(0).Metrics(), "mixed-type", pmetric.MetricTypeGauge)
	assert.True(t, foundHostAMixedGauge)
	assert.Equal(t, 2, hostAMixedGauge.Gauge().DataPoints().Len())
	hostAMixedSum, foundHostAMixedSum := retrieveMetric(hostA.ScopeMetrics().At(0).Metrics(), "mixed-type", pmetric.MetricTypeSum)
	assert.True(t, foundHostAMixedSum)
	assert.Equal(t, 2, hostAMixedSum.Sum().DataPoints().Len())

	// We must have host-B
	hostB, foundHostB := retrieveHostResource(processedMetrics.ResourceMetrics(), "host-B")
	assert.True(t, foundHostB)
	assert.Equal(t, 1, hostB.Resource().Attributes().Len())
	assert.Equal(t, 1, hostB.ScopeMetrics().Len())
	assert.Equal(t, 2, hostB.ScopeMetrics().At(0).Metrics().Len())
	hostBGauge1, foundHostBGauge1 := retrieveMetric(hostB.ScopeMetrics().At(0).Metrics(), "gauge-1", pmetric.MetricTypeGauge)
	assert.True(t, foundHostBGauge1)
	assert.Equal(t, 2, hostBGauge1.Gauge().DataPoints().Len())
	hostBMixedGauge, foundHostBMixedGauge := retrieveMetric(hostB.ScopeMetrics().At(0).Metrics(), "mixed-type", pmetric.MetricTypeGauge)
	assert.True(t, foundHostBMixedGauge)
	assert.Equal(t, 1, hostBMixedGauge.Gauge().DataPoints().Len())
}

// Test helper function that retrieves the resource with the specified "host.name" attribute
func retrieveHostResource(resources pmetric.ResourceMetricsSlice, hostname string) (pmetric.ResourceMetrics, bool) {
	for i := 0; i < resources.Len(); i++ {
		resource := resources.At(i)
		hostnameValue, foundKey := resource.Resource().Attributes().Get("host.name")
		if foundKey && hostnameValue.AsString() == hostname {
			return resource, true
		}
	}
	return pmetric.ResourceMetrics{}, false
}

// Test helper function that retrieves the specified metric
func retrieveMetric(metrics pmetric.MetricSlice, name string, metricType pmetric.MetricType) (pmetric.Metric, bool) {
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == name && metric.Type() == metricType {
			return metric, true
		}
	}
	return pmetric.Metric{}, false
}

func TestCompacting(t *testing.T) {
	spans := someSpans(attrMap, 10, 10)
	logs := someLogs(attrMap, 10, 10)
	metrics := someGaugeMetrics(attrMap, 10, 10)

	assert.Equal(t, 100, spans.ResourceSpans().Len())
	assert.Equal(t, 100, logs.ResourceLogs().Len())
	assert.Equal(t, 100, metrics.ResourceMetrics().Len())

	gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{})
	require.NoError(t, err)

	processedSpans, err := gap.processTraces(context.Background(), spans)
	assert.NoError(t, err)
	processedLogs, err := gap.processLogs(context.Background(), logs)
	assert.NoError(t, err)
	processedMetrics, err := gap.processMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	assert.Equal(t, 1, processedSpans.ResourceSpans().Len())
	assert.Equal(t, 1, processedLogs.ResourceLogs().Len())
	assert.Equal(t, 1, processedMetrics.ResourceMetrics().Len())

	rss := processedSpans.ResourceSpans().At(0)
	rls := processedLogs.ResourceLogs().At(0)
	rlm := processedMetrics.ResourceMetrics().At(0)

	assert.Equal(t, 0, rss.Resource().Attributes().Len())
	assert.Equal(t, 0, rls.Resource().Attributes().Len())
	assert.Equal(t, 0, rlm.Resource().Attributes().Len())

	assert.Equal(t, 10, rss.ScopeSpans().Len())
	assert.Equal(t, 10, rls.ScopeLogs().Len())
	assert.Equal(t, 10, rlm.ScopeMetrics().Len())

	for i := 0; i < 10; i++ {
		ils := rss.ScopeSpans().At(i)
		sl := rls.ScopeLogs().At(i)
		ilm := rlm.ScopeMetrics().At(i)

		assert.Equal(t, 10, ils.Spans().Len())
		assert.Equal(t, 10, sl.LogRecords().Len())
		assert.Equal(t, 10, ilm.Metrics().Len())
	}
}

func Test_GetMetricInInstrumentationLibrary(t *testing.T) {
	// input metric with datapoint
	m := pmetric.NewMetric()
	m.SetName("metric")
	m.SetDescription("description")
	m.SetUnit("unit")
	d := m.SetEmptyGauge().DataPoints().AppendEmpty()
	d.SetDoubleValue(1.0)

	// expected metric without datapoint
	// the datapoints are not copied to the resulting metric, since
	// datapoints are moved in between metrics in the processor
	m2 := pmetric.NewMetric()
	m2.SetName("metric")
	m2.SetDescription("description")
	m2.SetUnit("unit")
	m2.SetEmptyGauge()

	metadata := pcommon.NewMap()
	metadata.PutStr("key", "val")
	metadata.CopyTo(m.Metadata())
	metadata.CopyTo(m2.Metadata())

	sm := pmetric.NewScopeMetrics()
	m.CopyTo(sm.Metrics().AppendEmpty())

	tests := []struct {
		name     string
		ilm      pmetric.ScopeMetrics
		searched pmetric.Metric
		want     pmetric.Metric
	}{
		{
			name:     "existing metric",
			ilm:      sm,
			searched: m,
			want:     m,
		},
		{
			name:     "non-existing metric - datapoints will be removed",
			ilm:      pmetric.NewScopeMetrics(),
			searched: m,
			want:     m2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, getMetricInInstrumentationLibrary(tt.ilm, tt.searched))
		})
	}
}

func BenchmarkCompacting(bb *testing.B) {
	runs := []struct {
		ilCount   int
		spanCount int
	}{
		{
			ilCount:   1,
			spanCount: 100,
		},
		{
			ilCount:   10,
			spanCount: 10,
		},
		{
			ilCount:   100,
			spanCount: 1,
		},
	}

	for _, run := range runs {
		bb.Run(fmt.Sprintf("instrumentation_library_count=%d, spans_per_library_count=%d", run.ilCount, run.spanCount), func(b *testing.B) {
			spans := someSpans(attrMap, run.ilCount, run.spanCount)
			gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{})
			require.NoError(b, err)

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				_, err := gap.processTraces(context.Background(), spans)
				if err != nil {
					return
				}
			}
		})
	}
}
