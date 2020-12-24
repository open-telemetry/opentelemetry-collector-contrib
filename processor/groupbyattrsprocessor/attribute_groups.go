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

// spansHavingAttrs keeps a grouping attributes and matching span records
type spansHavingAttrs struct {
	attrs         pdata.AttributeMap
	resourceSpans pdata.ResourceSpans
}

// logsHavingAttrs keeps a grouping attributes and matching log records
type logsHavingAttrs struct {
	attrs        pdata.AttributeMap
	resourceLogs pdata.ResourceLogs
}

func instrumentationLibrariesEqual(il1, il2 pdata.InstrumentationLibrary) bool {
	return il1.Name() == il2.Name() && il1.Version() == il2.Version()
}

// matchingInstrumentationLibrarySpans searches for a pdata.InstrumentationLibrarySpans instance matching
// given InstrumentationLibrary. If nothing is found, it creates a new one
func (ae *spansHavingAttrs) matchingInstrumentationLibrarySpans(library pdata.InstrumentationLibrary) pdata.InstrumentationLibrarySpans {
	ilss := ae.resourceSpans.InstrumentationLibrarySpans()
	for i := 0; i < ilss.Len(); i++ {
		ill := ilss.At(i)
		if instrumentationLibrariesEqual(ill.InstrumentationLibrary(), library) {
			return ill
		}
	}

	ilss.Resize(ilss.Len() + 1)
	ils := ilss.At(ilss.Len() - 1)
	library.CopyTo(ils.InstrumentationLibrary())
	return ils
}

// matchingInstrumentationLibraryLogs searches for a pdata.InstrumentationLibraryLogs instance matching
// given InstrumentationLibrary. If nothing is found, it creates a new one
func (ae *logsHavingAttrs) matchingInstrumentationLibraryLogs(library pdata.InstrumentationLibrary) pdata.InstrumentationLibraryLogs {
	ills := ae.resourceLogs.InstrumentationLibraryLogs()
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
type spansGroupedByAttrs []spansHavingAttrs

// logsGroupedByAttrs keeps all found grouping attributes for logs, together with the matching records
type logsGroupedByAttrs []logsHavingAttrs

func newLogsGroupedByAttrs() *logsGroupedByAttrs {
	return &logsGroupedByAttrs{}
}

func newSpansGroupedByAttrs() *spansGroupedByAttrs {
	return &spansGroupedByAttrs{}
}

func (lag logsGroupedByAttrs) findGroup(attrs pdata.AttributeMap) *logsHavingAttrs {
	for i := 0; i < len(lag); i++ {
		if attributeMapsEqual(lag[i].attrs, attrs) {
			return &lag[i]
		}
	}
	return nil
}

func (sag spansGroupedByAttrs) findGroup(attrs pdata.AttributeMap) *spansHavingAttrs {
	for i := 0; i < len(sag); i++ {
		if attributeMapsEqual(sag[i].attrs, attrs) {
			return &sag[i]
		}
	}
	return nil
}

func attributeMapsEqual(attrs1 pdata.AttributeMap, attrs2 pdata.AttributeMap) bool {
	if attrs1.Len() != attrs2.Len() {
		return false
	}

	matching := true

	attrs1.ForEach(func(k1 string, v1 pdata.AttributeValue) {
		if matching {
			v2, found := attrs2.Get(k1)
			if !found || !v1.Equal(v2) {
				matching = false
			}
		}
	})

	return matching
}

// attributeGroup searches for a group with matching attributes and returns it, if nothing is found, it is being created
func (sag *spansGroupedByAttrs) attributeGroup(attrs pdata.AttributeMap, baseResource pdata.Resource) spansHavingAttrs {
	entry := sag.findGroup(attrs)
	if entry == nil {
		entry = &spansHavingAttrs{
			attrs:         attrs,
			resourceSpans: pdata.NewResourceSpans(),
		}

		baseResource.CopyTo(entry.resourceSpans.Resource())
		attrs.CopyTo(entry.resourceSpans.Resource().Attributes())

		*sag = append(*sag, *entry)
	}

	return *entry
}

// attributeGroup searches for a group with matching attributes and returns it, if nothing is found, it is being created
func (lag *logsGroupedByAttrs) attributeGroup(attrs pdata.AttributeMap, baseResource pdata.Resource) logsHavingAttrs {
	entry := lag.findGroup(attrs)
	if entry == nil {
		entry = &logsHavingAttrs{
			attrs:        attrs,
			resourceLogs: pdata.NewResourceLogs(),
		}

		baseResource.CopyTo(entry.resourceLogs.Resource())
		attrs.CopyTo(entry.resourceLogs.Resource().Attributes())

		*lag = append(*lag, *entry)
	}

	return *entry
}
