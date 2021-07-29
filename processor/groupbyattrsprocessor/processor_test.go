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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

var (
	attrMap = prepareAttributeMap()
)

func prepareAttributeMap() pdata.AttributeMap {
	attributeValues := map[string]pdata.AttributeValue{
		"xx": pdata.NewAttributeValueString("aa"),
		"yy": pdata.NewAttributeValueInt(11),
	}

	am := pdata.NewAttributeMap()
	am.InitFromMap(attributeValues)

	am.Sort()
	return am
}

func prepareResource(attrMap pdata.AttributeMap, selectedKeys []string) pdata.Resource {
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

func filterAttributeMap(attrMap pdata.AttributeMap, selectedKeys []string) pdata.AttributeMap {
	filteredAttrMap := pdata.NewAttributeMap()
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
			log := rl.InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
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
			span := rs.InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("foo-%d-%d", i, j))
			span.Attributes().InsertString("commonGroupedAttr", "abc")
			span.Attributes().InsertString("commonNonGroupedAttr", "xyz")
		}
	}

	return traces
}

// The "complex" use case has following input data:
//  * Resource[Spans|Logs] #1
//    Attributes: resourceAttrIndex => <resource_no> (when `withResourceAttrIndex` set to true)
//      * InstrumentationLibrary[Spans|Logs] #1
//          * [Span|Log] foo-1-1
//            Attributes: commonGroupedAttr => abc, commonNonGroupedAttr => xyz
//      * InstrumentationLibrary[Spans|Logs] #M
//        ...
//    ...
//   * Resource[Spans|Logs] #N
//      ...
func TestComplexAttributeGrouping(t *testing.T) {
	// Following are record-level attributes that should be preserved after processing
	outputRecordAttrs := pdata.NewAttributeMap()
	outputRecordAttrs.InsertString("commonNonGroupedAttr", "xyz")

	tests := []struct {
		name                              string
		withResourceAttrIndex             bool
		inputResourceCount                int
		inputInstrumentationLibraryCount  int
		outputResourceCount               int
		outputInstrumentationLibraryCount int // Per each Resource
		outputTotalRecordsCount           int // Per each Instrumentation Library
	}{
		{
			name:                             "With not unique Resource-level attributes",
			withResourceAttrIndex:            false,
			inputResourceCount:               4,
			inputInstrumentationLibraryCount: 4,
			// All resources and instrumentation libraries are matching and can be joined together
			outputResourceCount:               1,
			outputInstrumentationLibraryCount: 1,
			outputTotalRecordsCount:           16,
		},
		{
			name:                             "With unique Resource-level attributes",
			withResourceAttrIndex:            true,
			inputResourceCount:               4,
			inputInstrumentationLibraryCount: 4,
			// Since each resource has a unique attribute value, so they cannot be joined together into one
			outputResourceCount:               4,
			outputInstrumentationLibraryCount: 1,
			outputTotalRecordsCount:           16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputLogs := someComplexLogs(tt.withResourceAttrIndex, tt.inputResourceCount, tt.inputInstrumentationLibraryCount)
			inputTraces := someComplexTraces(tt.withResourceAttrIndex, tt.inputResourceCount, tt.inputInstrumentationLibraryCount)

			gap, err := createGroupByAttrsProcessor(zap.NewNop(), []string{"commonGroupedAttr"})
			require.NoError(t, err)

			processedLogs, err := gap.processLogs(context.Background(), inputLogs)
			assert.NoError(t, err)

			processedSpans, err := gap.processTraces(context.Background(), inputTraces)
			assert.NoError(t, err)

			rls := processedLogs.ResourceLogs()
			assert.Equal(t, tt.outputResourceCount, rls.Len())
			assert.Equal(t, tt.outputTotalRecordsCount, processedLogs.LogRecordCount())
			for i := 0; i < rls.Len(); i++ {
				rl := rls.At(i)
				assert.Equal(t, tt.outputInstrumentationLibraryCount, rl.InstrumentationLibraryLogs().Len())

				// This was present at record level and should be found on Resource level after the processor
				commonAttrValue, _ := rl.Resource().Attributes().Get("commonGroupedAttr")
				assert.Equal(t, pdata.NewAttributeValueString("abc"), commonAttrValue)

				for j := 0; j < rl.InstrumentationLibraryLogs().Len(); j++ {
					logs := rl.InstrumentationLibraryLogs().At(j).Logs()
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
				assert.Equal(t, tt.outputInstrumentationLibraryCount, rs.InstrumentationLibrarySpans().Len())

				// This was present at record level and should be found on Resource level after the processor
				commonAttrValue, _ := rs.Resource().Attributes().Get("commonGroupedAttr")
				assert.Equal(t, pdata.NewAttributeValueString("abc"), commonAttrValue)

				for j := 0; j < rs.InstrumentationLibrarySpans().Len(); j++ {
					spans := rs.InstrumentationLibrarySpans().At(j).Spans()
					for k := 0; k < spans.Len(); k++ {
						assert.EqualValues(t, outputRecordAttrs, spans.At(k).Attributes())
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
			name:           "One attribute",
			groupByKeys:    []string{"xx"},
			nonGroupedKeys: []string{"yy"},
			count:          4,
		},
		{
			name:           "No groupByKeys",
			groupByKeys:    []string{"zz"},
			nonGroupedKeys: []string{"xx", "yy"},
			count:          4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := someLogs(attrMap, tt.count)
			spans := someSpans(attrMap, tt.count)

			gap, err := createGroupByAttrsProcessor(zap.NewNop(), tt.groupByKeys)
			require.NoError(t, err)

			expectedResource := prepareResource(attrMap, tt.groupByKeys)
			expectedAttributes := filterAttributeMap(attrMap, tt.nonGroupedKeys)

			processedLogs, err := gap.processLogs(context.Background(), logs)
			assert.NoError(t, err)

			processedSpans, err := gap.processTraces(context.Background(), spans)
			assert.NoError(t, err)

			assert.Equal(t, 1, processedLogs.ResourceLogs().Len())
			assert.Equal(t, 1, processedSpans.ResourceSpans().Len())

			resources := []pdata.Resource{
				processedLogs.ResourceLogs().At(0).Resource(),
				processedSpans.ResourceSpans().At(0).Resource(),
			}

			for _, res := range resources {
				res.Attributes().Sort()
				assert.Equal(t, expectedResource, res)
			}

			ills := processedLogs.ResourceLogs().At(0).InstrumentationLibraryLogs()
			ilss := processedSpans.ResourceSpans().At(0).InstrumentationLibrarySpans()

			assert.Equal(t, 1, ills.Len())
			assert.Equal(t, 1, ilss.Len())

			ls := ills.At(0).Logs()
			ss := ilss.At(0).Spans()
			assert.Equal(t, tt.count, ls.Len())
			assert.Equal(t, tt.count, ss.Len())

			for i := 0; i < ls.Len(); i++ {
				log := ls.At(i)
				span := ss.At(i)

				log.Attributes().Sort()
				span.Attributes().Sort()

				assert.EqualValues(t, expectedAttributes, log.Attributes())
				assert.EqualValues(t, expectedAttributes, span.Attributes())
			}
		})
	}
}

func someSpans(attrs pdata.AttributeMap, count int) pdata.Traces {
	traces := pdata.NewTraces()
	ils := traces.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty()

	for i := 0; i < count; i++ {
		span := ils.Spans().AppendEmpty()
		span.SetName(fmt.Sprint("foo-", i))
		attrs.CopyTo(span.Attributes())
	}

	return traces
}

func someLogs(attrs pdata.AttributeMap, count int) pdata.Logs {
	logs := pdata.NewLogs()
	ill := logs.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty()

	for i := 0; i < count; i++ {
		log := ill.Logs().AppendEmpty()
		log.SetName(fmt.Sprint("foo-", i))
		attrs.CopyTo(log.Attributes())
	}

	return logs
}
