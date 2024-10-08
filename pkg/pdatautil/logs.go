// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"

import "go.opentelemetry.io/collector/pdata/plog"

// MoveLogRecordsWithContextIf calls f sequentially for each LogRecord present in the first plog.Logs.
// If f returns true, the element is removed from the first plog.Logs and added to the second plog.Logs.
// Notably, the Resource and Scope associated with the LogRecord are recreated in the second plog.Logs only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveLogRecordsWithContextIf(from, to plog.Logs, f func(plog.LogRecord) bool) {
	rls := from.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		if rls.Len() == 0 {
			continue // Don't remove empty
		}
		rl := rls.At(i)
		sls := rl.ScopeLogs()
		var rlCopy *plog.ResourceLogs
		for j := 0; j < sls.Len(); j++ {
			if sls.Len() == 0 {
				continue // Don't remove empty
			}
			sl := sls.At(j)
			lrs := sl.LogRecords()
			var slCopy *plog.ScopeLogs
			moveWithContextIf := func(lr plog.LogRecord) bool {
				if !f(lr) {
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
				lr.CopyTo(slCopy.LogRecords().AppendEmpty())
				return true
			}
			lrs.RemoveIf(moveWithContextIf)
		}
		sls.RemoveIf(func(sl plog.ScopeLogs) bool {
			return sl.LogRecords().Len() == 0
		})
	}
	rls.RemoveIf(func(rl plog.ResourceLogs) bool {
		return rl.ScopeLogs().Len() == 0
	})
}
