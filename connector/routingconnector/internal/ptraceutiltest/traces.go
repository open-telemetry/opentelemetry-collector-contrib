// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutiltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutiltest"

import "go.opentelemetry.io/collector/pdata/ptrace"

// TestTraces returns a ptrace.Traces with a uniform structure where resources, scopes, spans,
// and spanevents are identical across all instances, except for one identifying field.
//
// Identifying fields:
// - Resources have an attribute called "resourceName" with a value of "resourceN".
// - Scopes have a name with a value of "scopeN".
// - Spans have a name with a value of "spanN".
// - Span Events have an attribute "spanEventName" with a value of "spanEventN".
//
// Example: TestTraces("AB", "XYZ", "MN", "1234") returns:
//
//	resourceA, resourceB
//	    each with scopeX, scopeY, scopeZ
//	        each with spanM, spanN
//	            each with spanEvent1, spanEvent2, spanEvent3, spanEvent4
//
// Each byte in the input string is a unique ID for the corresponding element.
func NewTraces(resourceIDs, scopeIDs, spanIDs, spanEventIDs string) ptrace.Traces {
	td := ptrace.NewTraces()
	for resourceN := 0; resourceN < len(resourceIDs); resourceN++ {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("resourceName", "resource"+string(resourceIDs[resourceN]))
		for scopeN := 0; scopeN < len(scopeIDs); scopeN++ {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("scope" + string(scopeIDs[scopeN]))
			for spanN := 0; spanN < len(spanIDs); spanN++ {
				s := ss.Spans().AppendEmpty()
				s.SetName("span" + string(spanIDs[spanN]))
				for spanEventN := 0; spanEventN < len(spanEventIDs); spanEventN++ {
					se := s.Events().AppendEmpty()
					se.Attributes().PutStr("spanEventName", "spanEvent"+string(spanEventIDs[spanEventN]))
				}
			}
		}
	}
	return td
}
