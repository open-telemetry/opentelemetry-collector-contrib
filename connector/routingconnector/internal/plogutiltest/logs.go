// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogutiltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutiltest"

import "go.opentelemetry.io/collector/pdata/plog"

// TestLogs returns a plog.Logs with a uniform structure where resources, scopes, and
// log records are identical across all instances, except for one identifying field.
//
// Identifying fields:
// - Resources have an attribute called "resourceName" with a value of "resourceN".
// - Scopes have a name with a value of "scopeN".
// - LogRecords have a body with a value of "logN".
//
// Example: TestLogs("AB", "XYZ", "1234") returns:
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

type Resource struct {
	id     byte
	scopes []Scope
}

type Scope struct {
	id   byte
	logs string
}

func WithResource(id byte, scopes ...Scope) Resource {
	r := Resource{id: id}
	r.scopes = append(r.scopes, scopes...)
	return r
}

func WithScope(id byte, logs string) Scope {
	return Scope{id: id, logs: logs}
}

// NewLogsFromOpts creates a plog.Logs with the specified resources, scopes, and logs.
// The general idea is the same as NewLogs, but this function allows for more flexibility
// in creating non-uniform structures.
func NewLogsFromOpts(resources ...Resource) plog.Logs {
	ld := plog.NewLogs()
	for _, resource := range resources {
		r := ld.ResourceLogs().AppendEmpty()
		r.Resource().Attributes().PutStr("resourceName", "resource"+string(resource.id))
		for _, scope := range resource.scopes {
			s := r.ScopeLogs().AppendEmpty()
			s.Scope().SetName("scope" + string(scope.id))
			for i := 0; i < len(scope.logs); i++ {
				l := s.LogRecords().AppendEmpty()
				l.Body().SetStr("log" + string(scope.logs[i]))
			}
		}
	}
	return ld
}
