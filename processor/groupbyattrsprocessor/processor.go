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

type groupByAttrsProcessor struct {
	logger      *zap.Logger
	groupByKeys []string
}

// ProcessTraces process traces and groups traces by attribute.
func (gap *groupByAttrsProcessor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	extractedGroups := newSpansGroupedByAttrs()

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)

				groupedAnything, groupedAttrMap := gap.splitAttrMap(span.Attributes())
				if groupedAnything {
					mNumGroupedSpans.M(1)
					// Some attributes are going to be moved from span to resource level,
					// so we can delete those on the record level
					deleteAttributes(groupedAttrMap, span.Attributes())
				} else {
					mNumNonGroupedSpans.M(1)
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedSpans := extractedGroups.attributeGroup(rs.Resource(), groupedAttrMap)
				matchingInstrumentationLibrarySpans(groupedSpans, ils.InstrumentationLibrary()).Spans().Append(span)
			}
		}
	}

	// Copy the grouped data into output
	groupedTraces := pdata.NewTraces()
	groupedResourceSpans := groupedTraces.ResourceSpans()
	for _, eg := range *extractedGroups {
		groupedResourceSpans.Append(eg)
	}
	mDistSpanGroups.M(int64(len(*extractedGroups)))

	return groupedTraces, nil
}

func (gap *groupByAttrsProcessor) ProcessLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	rl := ld.ResourceLogs()
	extractedGroups := newLogsGroupedByAttrs()

	for i := 0; i < rl.Len(); i++ {
		ls := rl.At(i)

		ills := ls.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			for k := 0; k < ill.Logs().Len(); k++ {
				log := ill.Logs().At(k)

				groupedAnything, groupedAttrMap := gap.splitAttrMap(log.Attributes())
				if groupedAnything {
					mNumGroupedLogs.M(1)
					// Some attributes are going to be moved from log record to resource level,
					// so we can delete those on the record level
					deleteAttributes(groupedAttrMap, log.Attributes())
				} else {
					mNumNonGroupedLogs.M(1)
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedLogs := extractedGroups.attributeGroup(ls.Resource(), groupedAttrMap)
				matchingInstrumentationLibraryLogs(groupedLogs, ill.InstrumentationLibrary()).Logs().Append(log)
			}
		}

	}

	// Copy the grouped data into output
	groupedLogs := pdata.NewLogs()
	groupedResourceLogs := groupedLogs.ResourceLogs()
	for _, eg := range *extractedGroups {
		groupedResourceLogs.Append(eg)
	}
	mDistLogGroups.M(int64(len(*extractedGroups)))

	return groupedLogs, nil
}

func deleteAttributes(attrsForRemoval, targetAttrs pdata.AttributeMap) {
	attrsForRemoval.ForEach(func(key string, _ pdata.AttributeValue) {
		targetAttrs.Delete(key)
	})
}

// splitAttrMap splits the AttributeMap by groupByKeys and returns a tuple:
//  - the first element indicates if anything was matched (true) or nothing (false)
//  - the second element contains groupByKeys that match given keys
func (gap *groupByAttrsProcessor) splitAttrMap(attrMap pdata.AttributeMap) (bool, pdata.AttributeMap) {
	groupedAttrMap := pdata.NewAttributeMap()
	groupedAnything := false

	for _, attrKey := range gap.groupByKeys {
		attrVal, found := attrMap.Get(attrKey)
		if found {
			groupedAttrMap.Insert(attrKey, attrVal)
			groupedAnything = true
		}
	}

	return groupedAnything, groupedAttrMap
}
