// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// storage is an abstraction for the span storage used by the groupbytrace processor.
// Implementations should be safe for concurrent use.
type storage interface {
	// createOrAppend will check whether the given trace ID is already in the storage and
	// will either append the given spans to the existing record, or create a new trace with
	// the given spans from trace
	createOrAppend(pcommon.TraceID, ptrace.Traces) error

	// get will retrieve the trace based on the given trace ID, returning nil in case a trace
	// cannot be found
	get(pcommon.TraceID) ([]ptrace.ResourceSpans, error)

	// delete will remove the trace based on the given trace ID, returning the trace that was removed,
	// or nil in case a trace cannot be found
	delete(pcommon.TraceID) ([]ptrace.ResourceSpans, error)

	// start gives the storage the opportunity to initialize any resources or procedures
	start() error

	// shutdown signals the storage that the processor is shutting down
	shutdown() error
}
