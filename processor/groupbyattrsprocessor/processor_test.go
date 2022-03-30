// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbyattrsprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

var (
	attrMap = prepareAttributeMap()
)

func prepareAttributeMap() pdata.Map {
	attributeValues := map[string]interface{}{
		"xx": "aa",
		"yy": 11,
	}

	am := pdata.NewMap()
	pdata.NewMapFromRaw(attributeValues).CopyTo(am)

	am.Sort()
	return am
}

func prepareResource(attrMap pdata.Map, selectedKeys []string) pdata.Resource {
	res := pdata.NewResource()
	for _, key := range selectedKeys {
		val, found := attrMap.Get(key)
		if found {
			res.Attributes().Insert(key, val)
		}
	}
	res.Attributes().Sort()
	return res
}

func filterAttributeMap(attrMap pdata.Map, selectedKeys []string) pdata.Map {
	filteredAttrMap := pdata.NewMap()
	if len(selectedKeys) == 0 {
		return filteredAttrMap
	}

	filteredAttrMap.EnsureCapacity(10)
	for _, key := range selectedKeys {
		val, _ := attrMap.Get(key)
		filteredAttrMap.Insert(key, val)
	}
	filteredAttrMap.Sort()
	return filteredAttrMap
}

func someComplexLogs(withResourceAttrIndex bool, rlCount int, illCount int) pdata.Logs {
	logs := pdata.NewLogs()

	for i := 0; i < rlCount; i++ {
		rl := logs.ResourceLogs().AppendEmpty()
		if withResourceAttrIndex {
			rl.Resource().Attributes().InsertInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < illCount; j++ {
			log := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			log.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			log.Attributes().InsertString("commonGroupedAttr", "abc")
			log.Attributes().InsertString("commonNonGroupedAttr", "xyz")
		}
	}

	return logs
}

func someComplexTraces(withResourceAttrIndex bool, rsCount int, ilsCount int) pdata.Traces {
	traces := pdata.NewTraces()

	for i := 0; i < rsCount; i++ {
		rs := traces.ResourceSpans().AppendEmpty()
		if withResourceAttrIndex {
			rs.Resource().Attributes().InsertInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < ilsCount; j++ {
			span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			span.Attributes().InsertString("commonGroupedAttr", "abc")
			span.Attributes().InsertString("commonNonGroupedAttr", "xyz")
		}
	}

	return traces
}

func someComplexMetrics(withResourceAttrIndex bool, rmCount int, ilmCount int, dataPointCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()

	for i := 0; i < rmCount; i++ {
		rm := metrics.ResourceMetrics().AppendEmpty()
		if withResourceAttrIndex {
			rm.Resource().Attributes().InsertInt("resourceAttrIndex", int64(i))
		}

		for j := 0; j < ilmCount; j++ {
			metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			metric.SetDataType(pdata.MetricDataTypeGauge)

			for k := 0; k < dataPointCount; k++ {
				dataPoint := metric.Gauge().DataPoints().AppendEmpty()
				dataPoint.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
				dataPoint.SetIntVal(int64(k))
				dataPoint.Attributes().InsertString("commonGroupedAttr", "abc")
				dataPoint.Attributes().InsertString("commonNonGroupedAttr", "xyz")
			}
		}
	}

	return metrics
}

func assertResourceContainsAttributes(t *testing.T, resource pdata.Resource, attributeMap pdata.Map) {
	attributeMap.Range(func(k string, v pdata.Value) bool {
		rv, found := resource.Attributes().Get(k)
		assert.True(t, found)
		assert.Equal(t, v, rv)
		return true
	})
}

// The "complex" use case has following input data:
//  * Resource[Spans|Logs|Metrics] #1
//    Attributes: resourceAttrIndex => <resource_no> (when `withResourceAttrIndex` set to true)
//      * InstrumentationLibrary[Spans|Logs|Metrics] #1
//          * [Span|Log] foo-1-1
//            Attributes: commonGroupedAttr => abc, commonNonGroupedAttr => xyz
//          * Metric foo-1-1
//            * DataPoint #1
//              IntValue: 1
//              Attributes: commonGroupedAttr => abc, commonNonGroupedAttr => xyz
//      * InstrumentationLibrary[Spans|Logs|Metrics] #M
//        ...
//    ...
//   * Resource[Spans|Logs|Metrics] #N
//      ...
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

			gap := createGroupByAttrsProcessor(zap.NewNop(), tt.groupByKeys)

			processedLogs, err := gap.processLogs(context.Background(), inputLogs)
			assert.NoError(t, err)

			processedSpans, err := gap.processTraces(context.Background(), inputTraces)
			assert.NoError(t, err)

			processedMetrics, err := gap.processMetrics(context.Background(), inputMetrics)
			assert.NoError(t, err)

			// Following are record-level attributes that should be preserved after processing
			outputRecordAttrs := pdata.NewMap()
			outputResourceAttrs := pdata.NewMap()
			if tt.shouldMoveCommonGroupedAttr {
				// This was present at record level and should be found on Resource level after the processor
				outputResourceAttrs.InsertString("commonGroupedAttr", "abc")
			} else {
				outputRecordAttrs.InsertString("commonGroupedAttr", "abc")
			}
			outputRecordAttrs.InsertString("commonNonGroupedAttr", "xyz")

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

			gap := createGroupByAttrsProcessor(zap.NewNop(), tt.groupByKeys)

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

			resources := []pdata.Resource{
				processedLogs.ResourceLogs().At(0).Resource(),
				processedSpans.ResourceSpans().At(0).Resource(),
				processedGaugeMetrics.ResourceMetrics().At(0).Resource(),
				processedSumMetrics.ResourceMetrics().At(0).Resource(),
				processedSummaryMetrics.ResourceMetrics().At(0).Resource(),
				processedHistogramMetrics.ResourceMetrics().At(0).Resource(),
				processedExponentialHistogramMetrics.ResourceMetrics().At(0).Resource(),
			}

			for _, res := range resources {
				res.Attributes().Sort()
				assert.Equal(t, expectedResource, res)
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

				log.Attributes().Sort()
				span.Attributes().Sort()
				gaugeDataPoint.Attributes().Sort()
				sumDataPoint.Attributes().Sort()
				summaryDataPoint.Attributes().Sort()
				histogramDataPoint.Attributes().Sort()
				exponentialHistogramDataPoint.Attributes().Sort()

				assert.EqualValues(t, expectedAttributes, log.Attributes())
				assert.EqualValues(t, expectedAttributes, span.Attributes())
				assert.EqualValues(t, expectedAttributes, gaugeDataPoint.Attributes())
				assert.EqualValues(t, expectedAttributes, sumDataPoint.Attributes())
				assert.EqualValues(t, expectedAttributes, summaryDataPoint.Attributes())
				assert.EqualValues(t, expectedAttributes, histogramDataPoint.Attributes())
				assert.EqualValues(t, expectedAttributes, exponentialHistogramDataPoint.Attributes())
			}
		})
	}
}

func someSpans(attrs pdata.Map, instrumentationLibraryCount int, spanCount int) pdata.Traces {
	traces := pdata.NewTraces()
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

func someLogs(attrs pdata.Map, instrumentationLibraryCount int, logCount int) pdata.Logs {
	logs := pdata.NewLogs()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < logCount; j++ {
			sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
			sl.Scope().SetName(ilName)
			log := sl.LogRecords().AppendEmpty()
			log.SetName(fmt.Sprint("foo-", j))
			attrs.CopyTo(log.Attributes())
		}
	}
	return logs
}

func someGaugeMetrics(attrs pdata.Map, instrumentationLibraryCount int, metricCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("gauge-", j))
			metric.SetDataType(pdata.MetricDataTypeGauge)
			dataPoint := metric.Gauge().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someSumMetrics(attrs pdata.Map, instrumentationLibraryCount int, metricCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("sum-", j))
			metric.SetDataType(pdata.MetricDataTypeSum)
			dataPoint := metric.Sum().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someSummaryMetrics(attrs pdata.Map, instrumentationLibraryCount int, metricCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("summary-", j))
			metric.SetDataType(pdata.MetricDataTypeSummary)
			dataPoint := metric.Summary().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someHistogramMetrics(attrs pdata.Map, instrumentationLibraryCount int, metricCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("histogram-", j))
			metric.SetDataType(pdata.MetricDataTypeHistogram)
			dataPoint := metric.Histogram().DataPoints().AppendEmpty()
			attrs.CopyTo(dataPoint.Attributes())
		}
	}
	return metrics
}

func someExponentialHistogramMetrics(attrs pdata.Map, instrumentationLibraryCount int, metricCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()
	for i := 0; i < instrumentationLibraryCount; i++ {
		ilName := fmt.Sprint("ils-", i)

		for j := 0; j < metricCount; j++ {
			ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName(ilName)
			metric := ilm.Metrics().AppendEmpty()
			metric.SetName(fmt.Sprint("exponential-histogram-", j))
			metric.SetDataType(pdata.MetricDataTypeExponentialHistogram)
			dataPoint := metric.ExponentialHistogram().DataPoints().AppendEmpty()
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

	metrics := pdata.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resourceMetrics.Resource().Attributes().UpsertString("host.name", "localhost")

	ilm := resourceMetrics.ScopeMetrics().AppendEmpty()

	// gauge-1
	gauge1 := ilm.Metrics().AppendEmpty()
	gauge1.SetName("gauge-1")
	gauge1.SetDataType(pdata.MetricDataTypeGauge)
	datapoint := gauge1.Gauge().DataPoints().AppendEmpty()
	datapoint.Attributes().UpsertString("host.name", "host-A")
	datapoint.Attributes().UpsertString("id", "eth0")
	datapoint = gauge1.Gauge().DataPoints().AppendEmpty()
	datapoint.Attributes().UpsertString("host.name", "host-A")
	datapoint.Attributes().UpsertString("id", "eth0")
	datapoint = gauge1.Gauge().DataPoints().AppendEmpty()
	datapoint.Attributes().UpsertString("host.name", "host-B")
	datapoint.Attributes().UpsertString("id", "eth0")

	// Duplicate the same metric, with same name and same type
	gauge1.CopyTo(ilm.Metrics().AppendEmpty())

	// mixed-type (GAUGE)
	mixedType1 := ilm.Metrics().AppendEmpty()
	gauge1.CopyTo(mixedType1)
	mixedType1.SetName("mixed-type")

	// mixed-type (same name but different TYPE)
	mixedType2 := ilm.Metrics().AppendEmpty()
	mixedType2.SetName("mixed-type")
	mixedType2.SetDataType(pdata.MetricDataTypeSum)
	datapoint = mixedType2.Sum().DataPoints().AppendEmpty()
	datapoint.Attributes().UpsertString("host.name", "host-A")
	datapoint.Attributes().UpsertString("id", "eth0")
	datapoint = mixedType2.Sum().DataPoints().AppendEmpty()
	datapoint.Attributes().UpsertString("host.name", "host-A")
	datapoint.Attributes().UpsertString("id", "eth0")

	// dontmove (metric that will not move to another resource)
	dontmove := ilm.Metrics().AppendEmpty()
	dontmove.SetName("dont-move")
	dontmove.SetDataType(pdata.MetricDataTypeGauge)
	datapoint = dontmove.Gauge().DataPoints().AppendEmpty()
	datapoint.Attributes().UpsertString("id", "eth0")

	// Perform the test
	gap := createGroupByAttrsProcessor(zap.NewNop(), []string{"host.name"})

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
	assert.Equal(t, pdata.MetricDataTypeGauge, localhostMetric.DataType())

	// We must have host-A
	hostA, foundHostA := retrieveHostResource(processedMetrics.ResourceMetrics(), "host-A")
	assert.True(t, foundHostA)
	assert.Equal(t, 1, hostA.Resource().Attributes().Len())
	assert.Equal(t, 1, hostA.ScopeMetrics().Len())
	assert.Equal(t, 3, hostA.ScopeMetrics().At(0).Metrics().Len())
	hostAGauge1, foundHostAGauge1 := retrieveMetric(hostA.ScopeMetrics().At(0).Metrics(), "gauge-1", pdata.MetricDataTypeGauge)
	assert.True(t, foundHostAGauge1)
	assert.Equal(t, 4, hostAGauge1.Gauge().DataPoints().Len())
	assert.Equal(t, 1, hostAGauge1.Gauge().DataPoints().At(0).Attributes().Len())
	metricIDAttribute, foundMetricIDAttribute := hostAGauge1.Gauge().DataPoints().At(0).Attributes().Get("id")
	assert.True(t, foundMetricIDAttribute)
	assert.Equal(t, "eth0", metricIDAttribute.AsString())
	hostAMixedGauge, foundHostAMixedGauge := retrieveMetric(hostA.ScopeMetrics().At(0).Metrics(), "mixed-type", pdata.MetricDataTypeGauge)
	assert.True(t, foundHostAMixedGauge)
	assert.Equal(t, 2, hostAMixedGauge.Gauge().DataPoints().Len())
	hostAMixedSum, foundHostAMixedSum := retrieveMetric(hostA.ScopeMetrics().At(0).Metrics(), "mixed-type", pdata.MetricDataTypeSum)
	assert.True(t, foundHostAMixedSum)
	assert.Equal(t, 2, hostAMixedSum.Sum().DataPoints().Len())

	// We must have host-B
	hostB, foundHostB := retrieveHostResource(processedMetrics.ResourceMetrics(), "host-B")
	assert.True(t, foundHostB)
	assert.Equal(t, 1, hostB.Resource().Attributes().Len())
	assert.Equal(t, 1, hostB.ScopeMetrics().Len())
	assert.Equal(t, 2, hostB.ScopeMetrics().At(0).Metrics().Len())
	hostBGauge1, foundHostBGauge1 := retrieveMetric(hostB.ScopeMetrics().At(0).Metrics(), "gauge-1", pdata.MetricDataTypeGauge)
	assert.True(t, foundHostBGauge1)
	assert.Equal(t, 2, hostBGauge1.Gauge().DataPoints().Len())
	hostBMixedGauge, foundHostBMixedGauge := retrieveMetric(hostB.ScopeMetrics().At(0).Metrics(), "mixed-type", pdata.MetricDataTypeGauge)
	assert.True(t, foundHostBMixedGauge)
	assert.Equal(t, 1, hostBMixedGauge.Gauge().DataPoints().Len())
}

// Test helper function that retrieves the resource with the specified "host.name" attribute
func retrieveHostResource(resources pdata.ResourceMetricsSlice, hostname string) (pdata.ResourceMetrics, bool) {
	for i := 0; i < resources.Len(); i++ {
		resource := resources.At(i)
		hostnameValue, foundKey := resource.Resource().Attributes().Get("host.name")
		if foundKey && hostnameValue.AsString() == hostname {
			return resource, true
		}
	}
	return pdata.ResourceMetrics{}, false
}

// Test helper function that retrieves the specified metric
func retrieveMetric(metrics pdata.MetricSlice, name string, metricType pdata.MetricDataType) (pdata.Metric, bool) {
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == name && metric.DataType() == metricType {
			return metric, true
		}
	}
	return pdata.Metric{}, false
}

func TestCompacting(t *testing.T) {
	spans := someSpans(attrMap, 10, 10)
	logs := someLogs(attrMap, 10, 10)
	metrics := someGaugeMetrics(attrMap, 10, 10)

	assert.Equal(t, 100, spans.ResourceSpans().Len())
	assert.Equal(t, 100, logs.ResourceLogs().Len())
	assert.Equal(t, 100, metrics.ResourceMetrics().Len())

	gap := createGroupByAttrsProcessor(zap.NewNop(), []string{})

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
			gap := createGroupByAttrsProcessor(zap.NewNop(), []string{})

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
