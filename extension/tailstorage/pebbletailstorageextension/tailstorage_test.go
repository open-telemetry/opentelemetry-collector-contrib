// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TailStorage is duplicated for tests in this package only.
//
// TODO: Use tailsamplingprocessor TailStorage once it is public.
type TailStorage interface {
	Append(traceID pcommon.TraceID, rss ptrace.ResourceSpans)
	Take(traceID pcommon.TraceID) (ptrace.Traces, bool)
	Delete(traceID pcommon.TraceID)
}
