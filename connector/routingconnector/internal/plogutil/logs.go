// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutil"

import (
	"go.opentelemetry.io/collector/pdata/plog"
)

// MoveResourcesIf calls f sequentially for each ResourceLogs present in the first plog.Logs.
// If f returns true, the element is removed from the first plog.Logs and added to the second plog.Logs.
func MoveResourcesIf(from, to plog.Logs, f func(plog.ResourceLogs) bool) {
	from.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		if !f(rl) {
			return false
		}
		rl.MoveTo(to.ResourceLogs().AppendEmpty())
		return true
	})
}

// MoveRecordsWithContextIf calls f sequentially for each LogRecord present in the first plog.Logs.
// If f returns true, the element is removed from the first plog.Logs and added to the second plog.Logs.
// Notably, the Resource and Scope associated with the LogRecord are created in the second plog.Logs only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveRecordsWithContextIf(from, to plog.Logs, f func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool) {
	rls := from.ResourceLogs()
	rls.RemoveIf(func(rl plog.ResourceLogs) bool {
		sls := rl.ScopeLogs()
		var rlCopy *plog.ResourceLogs
		sls.RemoveIf(func(sl plog.ScopeLogs) bool {
			lrs := sl.LogRecords()
			var slCopy *plog.ScopeLogs
			lrs.RemoveIf(func(lr plog.LogRecord) bool {
				if !f(rl, sl, lr) {
					return false
				}
				if rlCopy == nil {
					rlc := to.ResourceLogs().AppendEmpty()
					rlCopy = &rlc
					rl.Resource().CopyTo(rlCopy.Resource())
					rlCopy.SetSchemaUrl(rl.SchemaUrl())
				}
				if slCopy == nil {
					slc := rlCopy.ScopeLogs().AppendEmpty()
					slCopy = &slc
					sl.Scope().CopyTo(slCopy.Scope())
					slCopy.SetSchemaUrl(sl.SchemaUrl())
				}
				lr.MoveTo(slCopy.LogRecords().AppendEmpty())
				return true
			})
			return sl.LogRecords().Len() == 0
		})
		return rl.ScopeLogs().Len() == 0
	})
}
