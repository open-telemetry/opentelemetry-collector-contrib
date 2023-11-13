// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// Log is a convenience struct for constructing logs for tests.
// See Logs for rationale.
type Log struct {
	Timestamp  int64
	Body       pcommon.Value
	Attributes map[string]any
}

// logConstructor is a convenience function for constructing logs for tests in a way that is
// relatively easy to read and write declaratively compared to the highly
// imperative and verbose method of using pdata directly.
// Attributes are sorted by key name.
func logConstructor(recs ...Log) plog.Logs {
	out := plog.NewLogs()
	logSlice := out.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	logSlice.EnsureCapacity(len(recs))
	for i := range recs {
		l := logSlice.AppendEmpty()
		recs[i].Body.CopyTo(l.Body())
		l.SetTimestamp(pcommon.Timestamp(recs[i].Timestamp))
		//nolint:errcheck
		l.Attributes().FromRaw(recs[i].Attributes)
	}

	return out
}
