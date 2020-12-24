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
	"go.opentelemetry.io/collector/consumer/pdata"
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
	filteredAttrMap.InitEmptyWithCapacity(10)
	for _, key := range selectedKeys {
		val, _ := attrMap.Get(key)
		filteredAttrMap.Insert(key, val)
	}
	filteredAttrMap.Sort()
	return filteredAttrMap
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

			gap, err := createGroupByAttrsProcessor(logger, tt.groupByKeys)
			require.NoError(t, err)

			expectedResource := prepareResource(attrMap, tt.groupByKeys)
			expectedAttributes := filterAttributeMap(attrMap, tt.nonGroupedKeys)

			processedLogs, err := gap.ProcessLogs(context.Background(), logs)
			assert.NoError(t, err)

			processedSpans, err := gap.ProcessTraces(context.Background(), spans)
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
	ils := pdata.NewInstrumentationLibrarySpans()

	for i := 0; i < count; i++ {
		span := pdata.NewSpan()
		span.SetName(fmt.Sprint("foo-", i))
		attrs.CopyTo(span.Attributes())

		ils.Spans().Append(span)
	}

	rs := pdata.NewResourceSpans()
	rs.InstrumentationLibrarySpans().Append(ils)

	traces := pdata.NewTraces()
	traces.ResourceSpans().Append(rs)

	return traces
}

func someLogs(attrs pdata.AttributeMap, count int) pdata.Logs {
	ill := pdata.NewInstrumentationLibraryLogs()

	for i := 0; i < count; i++ {
		log := pdata.NewLogRecord()
		log.SetName(fmt.Sprint("foo-", i))
		attrs.CopyTo(log.Attributes())

		ill.Logs().Append(log)
	}

	rl := pdata.NewResourceLogs()
	rl.InstrumentationLibraryLogs().Append(ill)

	logs := pdata.NewLogs()
	logs.ResourceLogs().Append(rl)

	return logs
}
