// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutil"

import "go.opentelemetry.io/collector/pdata/ptrace"

// MoveResourcesIf calls f sequentially for each ResourceSpans present in the first ptrace.Traces.
// If f returns true, the element is removed from the first ptrace.Traces and added to the second ptrace.Traces.
func MoveResourcesIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans) bool) {
	from.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		if !f(rs) {
			return false
		}
		rs.CopyTo(to.ResourceSpans().AppendEmpty())
		return true
	})
}
