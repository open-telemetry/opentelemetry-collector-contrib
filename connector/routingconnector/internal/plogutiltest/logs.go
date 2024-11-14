// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogutiltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutiltest"

import "go.opentelemetry.io/collector/pdata/plog"

// NewLogs returns a plog.Logs with a uniform structure where resources, scopes, and
// log records are identical across all instances, except for one identifying field.
//
// Identifying fields:
// - Resources have an attribute called "resourceName" with a value of "resourceN".
// - Scopes have a name with a value of "scopeN".
// - LogRecords have a body with a value of "logN".
//
// Example: NewLogs("AB", "XYZ", "1234") returns:
//
//	resourceA, resourceB
//	    each with scopeX, scopeY, scopeZ
//	        each with log1, log2, log3, log4
//
// Each byte in the input string is a unique ID for the corresponding element.
func NewLogs(resourceIDs, scopeIDs, logRecordIDs string) plog.Logs {
	ld := plog.NewLogs()
	for resourceN := 0; resourceN < len(resourceIDs); resourceN++ {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("resourceName", "resource"+string(resourceIDs[resourceN]))
		for scopeN := 0; scopeN < len(scopeIDs); scopeN++ {
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName("scope" + string(scopeIDs[scopeN]))
			for logRecordN := 0; logRecordN < len(logRecordIDs); logRecordN++ {
				lr := sl.LogRecords().AppendEmpty()
				lr.Body().SetStr("log" + string(logRecordIDs[logRecordN]))
			}
		}
	}
	return ld
}

func New(resources ...plog.ResourceLogs) plog.Logs {
	ld := plog.NewLogs()
	for _, resource := range resources {
		resource.CopyTo(ld.ResourceLogs().AppendEmpty())
	}
	return ld
}

func Resource(id string, scopes ...plog.ScopeLogs) plog.ResourceLogs {
	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("resourceName", "resource"+id)
	for _, scope := range scopes {
		scope.CopyTo(rl.ScopeLogs().AppendEmpty())
	}
	return rl
}

func Scope(id string, logs ...plog.LogRecord) plog.ScopeLogs {
	s := plog.NewScopeLogs()
	s.Scope().SetName("scope" + id)
	for _, log := range logs {
		log.CopyTo(s.LogRecords().AppendEmpty())
	}
	return s
}

func LogRecord(id string) plog.LogRecord {
	lr := plog.NewLogRecord()
	lr.Body().SetStr("log" + id)
	return lr
}
