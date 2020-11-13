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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type groupbyattrsprocessor struct {
	logger      *zap.Logger
	groupByKeys []string
}

// ProcessTraces process traces and groups traces by attribute.
func (gap *groupbyattrsprocessor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		extractedResourceSpans := newSpanAttributeGroups()

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)

				groupedAnything, groupedAttrMap, nonGroupedAttrMap := gap.splitAttrMap(span.Attributes())
				if groupedAnything {
					mNumGroupedSpans.M(1)
					overwriteAttributes(nonGroupedAttrMap, span.Attributes())
				} else {
					mNumNonGroupedSpans.M(1)
				}

				entry := extractedResourceSpans.attributeEntry(groupedAttrMap, rs.Resource())
				entry.instrumentationLibrarySpans(ils.InstrumentationLibrary()).Spans().Append(span)
			}
		}

		for _, ers := range extractedResourceSpans.entries {
			ers.resourceSpans.CopyTo(rs)
		}
		mDistSpanGroups.M(int64(len(extractedResourceSpans.entries)))
	}

	return td, nil
}

func (gap *groupbyattrsprocessor) ProcessLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	rl := ld.ResourceLogs()

	for i := 0; i < rl.Len(); i++ {
		ls := rl.At(i)

		extractedResourceLogs := newLogAttributeGroups()

		ills := ls.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			for k := 0; k < ill.Logs().Len(); k++ {
				log := ill.Logs().At(k)

				groupedAnything, groupedAttrMap, nonGroupedAttrMap := gap.splitAttrMap(log.Attributes())
				if groupedAnything {
					mNumGroupedLogs.M(1)
					overwriteAttributes(nonGroupedAttrMap, log.Attributes())
				} else {
					mNumNonGroupedLogs.M(1)
				}

				entry := extractedResourceLogs.attributeEntry(groupedAttrMap, ls.Resource())
				entry.instrumentationLibraryLogs(ill.InstrumentationLibrary()).Logs().Append(log)
			}
		}

		for _, erl := range extractedResourceLogs.entries {
			erl.resourceLogs.CopyTo(ls)
		}
		mDistLogGroups.M(int64(len(extractedResourceLogs.entries)))
	}

	return ld, nil
}

func overwriteAttributes(selectedAttrs, targetAttrs pdata.AttributeMap) {
	targetAttrs.InitEmptyWithCapacity(selectedAttrs.Len())
	selectedAttrs.CopyTo(targetAttrs)
}

// splitAttrMap splits the AttributeMap by groupByKeys and returns three-tuple:
//  - the first element indicates if anything was matched (true) or nothing (false)
//  - the second element contains groupByKeys that did not match the keys
//  - the third element contains groupByKeys that match given keys
func (gap *groupbyattrsprocessor) splitAttrMap(attrMap pdata.AttributeMap) (bool, pdata.AttributeMap, pdata.AttributeMap) {
	groupedAttrMap := pdata.NewAttributeMap()
	nonGroupedAttrMap := pdata.NewAttributeMap()

	groupedAnything := false

	for _, attrKey := range gap.groupByKeys {
		attrVal, found := attrMap.Get(attrKey)
		if found {
			groupedAttrMap.Insert(attrKey, attrVal)
			groupedAnything = true
		}
	}

	if !groupedAnything {
		return groupedAnything, groupedAttrMap, attrMap
	}

	groupedAttrMap.Sort()

	attrMap.ForEach(func(k string, v pdata.AttributeValue) {
		_, selected := groupedAttrMap.Get(k)
		if !selected {
			nonGroupedAttrMap.Insert(k, v)
		}
	})

	return groupedAnything, groupedAttrMap, nonGroupedAttrMap
}
