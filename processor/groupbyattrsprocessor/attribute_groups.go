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

// attributeEntry keeps a grouping attributes and matching records
type attributeEntry struct {
	attrs         pdata.AttributeMap
	resourceSpans pdata.ResourceSpans
	resourceLogs  pdata.ResourceLogs
}

func instrumentationLibrariesEqual(il1, il2 pdata.InstrumentationLibrary) bool {
	return il1.Name() == il2.Name() && il1.Version() == il2.Version()
}

// instrumentationLibrarySpans searches for a pdata.InstrumentationLibrarySpans instance matching
// given InstrumentatonLibrary. If nothing is found, it creates a new one
func (ae *attributeEntry) instrumentationLibrarySpans(library pdata.InstrumentationLibrary) pdata.InstrumentationLibrarySpans {
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

// instrumentationLibraryLogs searches for a pdata.InstrumentationLibraryLogs instance matching
// given InstrumentatonLibrary. If nothing is found, it creates a new one
func (ae *attributeEntry) instrumentationLibraryLogs(library pdata.InstrumentationLibrary) pdata.InstrumentationLibraryLogs {
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

// attributeGroups keeps all found grouping attributes, together with the matching records
type attributeGroups struct {
	entries []attributeEntry
}

type logAttributeGroups struct {
	attributeGroups
}

func newLogAttributeGroups() *logAttributeGroups {
	return &logAttributeGroups{
		attributeGroups{
			entries: []attributeEntry{},
		},
	}
}

type spanAttributeGroups struct {
	attributeGroups
}

func newSpanAttributeGroups() *spanAttributeGroups {
	return &spanAttributeGroups{
		attributeGroups{
			entries: []attributeEntry{},
		},
	}
}

func (ag *attributeGroups) findAttr(attrs pdata.AttributeMap) *attributeEntry {
	for i := 0; i < len(ag.entries); i++ {
		if attributeMapsEqual(ag.entries[i].attrs, attrs) {
			return &ag.entries[i]
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
			if found {
				if !v1.Equal(v2) {
					matching = false
				}
			} else {
				matching = false
			}
		}
	})

	return matching
}

// attributeEntry searches for an entry with matching attributes and returns it, if nothing is found, it is being created
func (sag *spanAttributeGroups) attributeEntry(attrs pdata.AttributeMap, res pdata.Resource) attributeEntry {
	entry := sag.findAttr(attrs)
	if entry == nil {
		entry = &attributeEntry{
			attrs:         attrs,
			resourceSpans: pdata.NewResourceSpans(),
		}

		res.CopyTo(entry.resourceSpans.Resource())
		attrs.CopyTo(entry.resourceSpans.Resource().Attributes())

		sag.entries = append(sag.entries, *entry)
	}

	return *entry
}

func (lag *logAttributeGroups) attributeEntry(attrs pdata.AttributeMap, res pdata.Resource) attributeEntry {
	entry := lag.findAttr(attrs)
	if entry == nil {
		entry = &attributeEntry{
			attrs:        attrs,
			resourceLogs: pdata.NewResourceLogs(),
		}

		res.CopyTo(entry.resourceLogs.Resource())
		attrs.CopyTo(entry.resourceLogs.Resource().Attributes())

		lag.entries = append(lag.entries, *entry)
	}

	return *entry
}
