// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TailStorage stores span batches keyed by trace ID for tail-sampling decisions.
// This interface is in development and subject to change.
//
// Concurrency contract:
//   - TailStorage does not require implementations to provide internal locking.
//   - Implementations may add internal synchronization, but it is optional.
//   - Caller e.g., tailsamplingprocessor is responsible for serialization at least per trace ID.
type TailStorage interface {
	// Append adds trace data for traceID to storage.
	// td should not be used after calling Append as
	// the storage implementation may mutate td for performance reasons.
	Append(traceID pcommon.TraceID, td ptrace.Traces) error

	// Take returns and removes all stored spans for a trace ID.
	//
	// If the trace exists, a non-empty ptrace.Traces is returned.
	// A subsequent Take for the same traceID must return an empty ptrace.Traces
	// until Append is called again for that traceID.
	Take(traceID pcommon.TraceID) (ptrace.Traces, error)

	// Delete removes any currently stored payload for traceID.
	//
	// error should only be non-nil if the operation itself failed due to any reason other than traceID not existing.
	// Delete must be idempotent. After Delete returns, Take(traceID) must return
	// an empty ptrace.Traces until Append is called again for traceID.
	Delete(traceID pcommon.TraceID) error
}
