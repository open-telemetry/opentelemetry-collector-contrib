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
	"go.opentelemetry.io/collector/consumer/pdata"
)

func instrumentationLibrariesEqual(il1, il2 pdata.InstrumentationLibrary) bool {
	return il1.Name() == il2.Name() && il1.Version() == il2.Version()
}

// matchingInstrumentationLibrarySpans searches for a pdata.InstrumentationLibrarySpans instance matching
// given InstrumentationLibrary. If nothing is found, it creates a new one
func matchingInstrumentationLibrarySpans(rl pdata.ResourceSpans, library pdata.InstrumentationLibrary) pdata.InstrumentationLibrarySpans {
	ilss := rl.InstrumentationLibrarySpans()
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		if instrumentationLibrariesEqual(ils.InstrumentationLibrary(), library) {
			return ils
		}
	}

	ilss.Resize(ilss.Len() + 1)
	ils := ilss.At(ilss.Len() - 1)
	library.CopyTo(ils.InstrumentationLibrary())
	return ils
}

// matchingInstrumentationLibraryLogs searches for a pdata.InstrumentationLibraryLogs instance matching
// given InstrumentationLibrary. If nothing is found, it creates a new one
func matchingInstrumentationLibraryLogs(rl pdata.ResourceLogs, library pdata.InstrumentationLibrary) pdata.InstrumentationLibraryLogs {
	ills := rl.InstrumentationLibraryLogs()
	for i := 0; i < ills.Len(); i++ {
		ill := ills.At(i)
		if instrumentationLibrariesEqual(ill.InstrumentationLibrary(), library) {
			return ill
		}
	}

	ills.Resize(ills.Len() + 1)
	ill := ills.At(ills.Len() - 1)
	library.CopyTo(ill.InstrumentationLibrary())
	return ill
}

// spansGroupedByAttrs keeps all found grouping attributes for spans, together with the matching records
type spansGroupedByAttrs []pdata.ResourceSpans

// logsGroupedByAttrs keeps all found grouping attributes for logs, together with the matching records
type logsGroupedByAttrs []pdata.ResourceLogs

func newLogsGroupedByAttrs() *logsGroupedByAttrs {
	return &logsGroupedByAttrs{}
}

func newSpansGroupedByAttrs() *spansGroupedByAttrs {
	return &spansGroupedByAttrs{}
}

// findGroup searches for an existing pdata.ResourceLogs that contains both the grouped attributes
// and base resource attributes. Returns the matching pdata.ResourceLogs and bool value which is set to true if found
func (lgba logsGroupedByAttrs) findGroup(baseResource pdata.Resource, attrs pdata.AttributeMap) (pdata.ResourceLogs, bool) {
	for i := 0; i < len(lgba); i++ {
		if resourceMatches(lgba[i].Resource(), baseResource, attrs) {
			return lgba[i], true
		}
	}
	return pdata.ResourceLogs{}, false
}

// findGroup searches for an existing pdata.ResourceLogs that contains both the grouped attributes
// and base resource attributes. Returns the matching pdata.ResourceLogs and bool value which is set to true if found
func (sgba spansGroupedByAttrs) findGroup(baseResource pdata.Resource, attrs pdata.AttributeMap) (pdata.ResourceSpans, bool) {
	for i := 0; i < len(sgba); i++ {
		if resourceMatches(sgba[i].Resource(), baseResource, attrs) {
			return sgba[i], true
		}
	}
	return pdata.ResourceSpans{}, false
}

// resourceMatches verifies if given pdata.Resource matches a composition of another (base) resource and attributes
func resourceMatches(res pdata.Resource, baseResource pdata.Resource, recordAttrs pdata.AttributeMap) bool {
	baseAttrs := baseResource.Attributes()

	// Some attributes in baseResource and recordAttrs might overlap, lets check obvious condition first before iterating
	minCommonAttrs := baseAttrs.Len() - recordAttrs.Len()
	if minCommonAttrs < 0 {
		minCommonAttrs = recordAttrs.Len() - baseAttrs.Len()
	}
	maxCommonAttrs := baseAttrs.Len() + recordAttrs.Len()
	if res.Attributes().Len() > maxCommonAttrs || res.Attributes().Len() < minCommonAttrs {
		return false
	}

	matching := true
	matchedBaseAttrs := 0
	matchedRecordAttrs := 0

	res.Attributes().ForEach(func(k1 string, v1 pdata.AttributeValue) {
		if matching {
			// Prioritize span-level attributes over resource attributes
			v2, recordAttrFound := recordAttrs.Get(k1)
			if recordAttrFound {
				matchedRecordAttrs++
				if !v1.Equal(v2) {
					matching = false
					return
				}
			}

			v2, baseAttrFound := baseAttrs.Get(k1)
			if baseAttrFound {
				matchedBaseAttrs++
				if !v1.Equal(v2) {
					matching = false
					return
				}
			}

			if !recordAttrFound && !baseAttrFound {
				matching = false
			}
		}
	})

	if matchedBaseAttrs != baseAttrs.Len() || matchedRecordAttrs != recordAttrs.Len() {
		return false
	}

	return matching
}

// attributeGroup searches for a group with matching attributes and returns it. If nothing is found, it is being created
func (sgba *spansGroupedByAttrs) attributeGroup(baseResource pdata.Resource, recordAttrs pdata.AttributeMap) pdata.ResourceSpans {
	res, found := sgba.findGroup(baseResource, recordAttrs)
	if !found {
		res = pdata.NewResourceSpans()

		baseResource.CopyTo(res.Resource())

		// This prioritizes span attributes over resource attributes, if they overlap
		attrs := res.Resource().Attributes()
		recordAttrs.ForEach(func(k string, v pdata.AttributeValue) {
			attrs.Upsert(k, v)
		})

		*sgba = append(*sgba, res)
	}

	return res
}

// attributeGroup searches for a group with matching attributes and returns it. If nothing is found, it is being created
func (lgba *logsGroupedByAttrs) attributeGroup(baseResource pdata.Resource, recordAttrs pdata.AttributeMap) pdata.ResourceLogs {
	res, found := lgba.findGroup(baseResource, recordAttrs)
	if !found {
		res = pdata.NewResourceLogs()
		baseResource.CopyTo(res.Resource())

		// This prioritizes log attributes over resource attributes, if they overlap
		attrs := res.Resource().Attributes()
		recordAttrs.ForEach(func(k string, v pdata.AttributeValue) {
			attrs.Upsert(k, v)
		})

		*lgba = append(*lgba, res)
	}

	return res
}
