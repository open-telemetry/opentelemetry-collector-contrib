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

	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type groupByAttrsProcessor struct {
	logger      *zap.Logger
	groupByKeys []string
}

// ProcessTraces process traces and groups traces by attribute.
func (gap *groupByAttrsProcessor) processTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
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
					stats.Record(ctx, mNumGroupedSpans.M(1))
					// Some attributes are going to be moved from span to resource level,
					// so we can delete those on the record level
					deleteAttributes(groupedAttrMap, span.Attributes())
				} else {
					stats.Record(ctx, mNumNonGroupedSpans.M(1))
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedSpans := extractedGroups.attributeGroup(rs.Resource(), groupedAttrMap)
				sp := matchingInstrumentationLibrarySpans(groupedSpans, ils.InstrumentationLibrary()).Spans().AppendEmpty()
				span.CopyTo(sp)
			}
		}
	}

	// Copy the grouped data into output
	groupedTraces := pdata.NewTraces()
	extractedGroups.MoveAndAppendTo(groupedTraces.ResourceSpans())
	stats.Record(ctx, mDistSpanGroups.M(int64(groupedTraces.ResourceSpans().Len())))

	return groupedTraces, nil
}

func (gap *groupByAttrsProcessor) processLogs(ctx context.Context, ld pdata.Logs) (pdata.Logs, error) {
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
					stats.Record(ctx, mNumGroupedLogs.M(1))
					// Some attributes are going to be moved from log record to resource level,
					// so we can delete those on the record level
					deleteAttributes(groupedAttrMap, log.Attributes())
				} else {
					stats.Record(ctx, mNumNonGroupedLogs.M(1))
				}

				// Lets combine the base resource attributes + the extracted (grouped) attributes
				// and keep them in the grouping entry
				groupedLogs := extractedGroups.attributeGroup(ls.Resource(), groupedAttrMap)
				lr := matchingInstrumentationLibraryLogs(groupedLogs, ill.InstrumentationLibrary()).Logs().AppendEmpty()
				log.CopyTo(lr)
			}
		}

	}

	// Copy the grouped data into output
	groupedLogs := pdata.NewLogs()
	extractedGroups.MoveAndAppendTo(groupedLogs.ResourceLogs())
	stats.Record(ctx, mDistLogGroups.M(int64(groupedLogs.ResourceLogs().Len())))

	return groupedLogs, nil
}

func deleteAttributes(attrsForRemoval, targetAttrs pdata.AttributeMap) {
	attrsForRemoval.Range(func(key string, _ pdata.AttributeValue) bool {
		targetAttrs.Delete(key)
		return true
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
