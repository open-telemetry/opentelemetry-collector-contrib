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

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func instrumentationLibrariesEqual(il1, il2 pcommon.InstrumentationScope) bool {
	return il1.Name() == il2.Name() && il1.Version() == il2.Version()
}

// matchingScopeSpans searches for a ptrace.ScopeSpans instance matching
// given InstrumentationScope. If nothing is found, it creates a new one
func matchingScopeSpans(rl ptrace.ResourceSpans, library pcommon.InstrumentationScope) ptrace.ScopeSpans {
	ilss := rl.ScopeSpans()
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		if instrumentationLibrariesEqual(ils.Scope(), library) {
			return ils
		}
	}

	ils := ilss.AppendEmpty()
	library.CopyTo(ils.Scope())
	return ils
}

// matchingScopeLogs searches for a plog.ScopeLogs instance matching
// given InstrumentationScope. If nothing is found, it creates a new one
func matchingScopeLogs(rl plog.ResourceLogs, library pcommon.InstrumentationScope) plog.ScopeLogs {
	ills := rl.ScopeLogs()
	for i := 0; i < ills.Len(); i++ {
		sl := ills.At(i)
		if instrumentationLibrariesEqual(sl.Scope(), library) {
			return sl
		}
	}

	sl := ills.AppendEmpty()
	library.CopyTo(sl.Scope())
	return sl
}

// matchingScopeMetrics searches for a pmetric.ScopeMetrics instance matching
// given InstrumentationScope. If nothing is found, it creates a new one
func matchingScopeMetrics(rm pmetric.ResourceMetrics, library pcommon.InstrumentationScope) pmetric.ScopeMetrics {
	ilms := rm.ScopeMetrics()
	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)
		if instrumentationLibrariesEqual(ilm.Scope(), library) {
			return ilm
		}
	}

	ilm := ilms.AppendEmpty()
	library.CopyTo(ilm.Scope())
	return ilm
}

// Build the Attributes that we'll be looking for in existing Resources as a merge of the Attributes
// of the original Resource with the requested Attributes
func buildReferenceAttributes(originResource pcommon.Resource, requiredAttributes pcommon.Map) pcommon.Map {
	referenceAttributes := pcommon.NewMap()
	originResource.Attributes().CopyTo(referenceAttributes)
	requiredAttributes.Range(func(k string, v pcommon.Value) bool {
		v.CopyTo(referenceAttributes.PutEmpty(k))
		return true
	})
	return referenceAttributes
}

// resourceMatches verifies if given pcommon.Resource attributes strictly match with the specified
// reference Attributes (all attributes must match strictly)
func resourceMatches(resource pcommon.Resource, referenceAttributes pcommon.Map) bool {

	// If not the same number of attributes, it doesn't match
	if referenceAttributes.Len() != resource.Attributes().Len() {
		return false
	}

	// Go through each attribute and check the corresponding attribute value in the tested Resource
	matching := true
	referenceAttributes.Range(func(referenceKey string, referenceValue pcommon.Value) bool {
		testedValue, foundKey := resource.Attributes().Get(referenceKey)
		if !foundKey || !referenceValue.Equal(testedValue) {
			// One difference is enough to consider it doesn't match, so fail early
			matching = false
			return false
		}
		return true
	})

	return matching
}

// Update the specified (and new) Resource with the properties of the original Resource, and with the
// required Attributes
func updateResourceToMatch(newResource pcommon.Resource, originResource pcommon.Resource, requiredAttributes pcommon.Map) {
	originResource.CopyTo(newResource)

	// This prioritizes required attributes over the original resource attributes, if they overlap
	attrs := newResource.Attributes()
	requiredAttributes.Range(func(k string, v pcommon.Value) bool {
		v.CopyTo(attrs.PutEmpty(k))
		return true
	})
}

// findOrCreateResource searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func findOrCreateResourceSpans(traces ptrace.Traces, originResource pcommon.Resource, requiredAttributes pcommon.Map) ptrace.ResourceSpans {

	// Build the reference attributes that we're looking for in Resources
	referenceAttributes := buildReferenceAttributes(originResource, requiredAttributes)

	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		if resourceMatches(rss.At(i).Resource(), referenceAttributes) {
			return rss.At(i)
		}
	}

	// Not found: append a new ResourceSpans.
	rs := rss.AppendEmpty()
	updateResourceToMatch(rs.Resource(), originResource, requiredAttributes)
	return rs
}

// findOrCreateResourceLogs searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func findOrCreateResourceLogs(logs plog.Logs, originResource pcommon.Resource, requiredAttributes pcommon.Map) plog.ResourceLogs {

	// Build the reference attributes that we're looking for in Resources
	referenceAttributes := buildReferenceAttributes(originResource, requiredAttributes)

	rls := logs.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		if resourceMatches(rls.At(i).Resource(), referenceAttributes) {
			return rls.At(i)
		}
	}

	// Not found: append a new ResourceLogs
	rl := rls.AppendEmpty()
	updateResourceToMatch(rl.Resource(), originResource, requiredAttributes)
	return rl
}

// findOrCreateResourceMetrics searches for a Resource with matching attributes and returns it. If nothing is found, it is being created
func findOrCreateResourceMetrics(metrics pmetric.Metrics, originResource pcommon.Resource, requiredAttributes pcommon.Map) pmetric.ResourceMetrics {

	// Build the reference attributes that we're looking for in Resources
	referenceAttributes := buildReferenceAttributes(originResource, requiredAttributes)

	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		if resourceMatches(rms.At(i).Resource(), referenceAttributes) {
			return rms.At(i)
		}
	}

	// Not found: append a new ResourceMetrics
	rm := rms.AppendEmpty()
	updateResourceToMatch(rm.Resource(), originResource, requiredAttributes)
	return rm
}
