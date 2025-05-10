// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestMoveResourcesIf(t *testing.T) {
	testCases := []struct {
		name       string
		moveIf     func(plog.ResourceLogs) bool
		from       plog.Logs
		to         plog.Logs
		expectFrom plog.Logs
		expectTo   plog.Logs
	}{
		{
			name: "move_none",
			moveIf: func(plog.ResourceLogs) bool {
				return false
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectTo:   plog.NewLogs(),
		},
		{
			name: "move_all",
			moveIf: func(plog.ResourceLogs) bool {
				return true
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plog.NewLogs(),
			expectTo:   plogutiltest.NewLogs("AB", "CD", "EF"),
		},
		{
			name: "move_one",
			moveIf: func(rl plog.ResourceLogs) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA"
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plogutiltest.NewLogs("B", "CD", "EF"),
			expectTo:   plogutiltest.NewLogs("A", "CD", "EF"),
		},
		{
			name: "move_to_preexisting",
			moveIf: func(rl plog.ResourceLogs) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plogutiltest.NewLogs("1", "2", "3"),
			expectFrom: plogutiltest.NewLogs("A", "CD", "EF"),
			expectTo: plogutiltest.NewLogsFromOpts(
				plogutiltest.WithResource('1', plogutiltest.WithScope('2', "3")),
				plogutiltest.WithResource('B', plogutiltest.WithScope('C', "EF"), plogutiltest.WithScope('D', "EF")),
			),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			plogutil.MoveResourcesIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, plogtest.CompareLogs(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, plogtest.CompareLogs(tt.expectTo, tt.to), "to not as expected")
		})
	}
}

func TestMoveRecordsWithContextIf(t *testing.T) {
	testCases := []struct {
		name       string
		moveIf     func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool
		from       plog.Logs
		to         plog.Logs
		expectFrom plog.Logs
		expectTo   plog.Logs
	}{
		{
			name: "move_none",
			moveIf: func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool {
				return false
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plogutiltest.NewLogs("AB", "CD", "EF"),
			expectTo:   plog.NewLogs(),
		},
		{
			name: "move_all",
			moveIf: func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool {
				return true
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plog.NewLogs(),
			expectTo:   plogutiltest.NewLogs("AB", "CD", "EF"),
		},
		{
			name: "move_all_from_one_resource",
			moveIf: func(rl plog.ResourceLogs, _ plog.ScopeLogs, _ plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plogutiltest.NewLogs("A", "CD", "EF"),
			expectTo:   plogutiltest.NewLogs("B", "CD", "EF"),
		},
		{
			name: "move_all_from_one_scope",
			moveIf: func(rl plog.ResourceLogs, sl plog.ScopeLogs, _ plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeC"
			},
			from: plogutiltest.NewLogs("AB", "CD", "EF"),
			to:   plog.NewLogs(),
			expectFrom: plogutiltest.NewLogsFromOpts(
				plogutiltest.WithResource('A', plogutiltest.WithScope('C', "EF"), plogutiltest.WithScope('D', "EF")),
				plogutiltest.WithResource('B', plogutiltest.WithScope('D', "EF")),
			),
			expectTo: plogutiltest.NewLogs("B", "C", "EF"),
		},
		{
			name: "move_all_from_one_scope_in_each_resource",
			moveIf: func(_ plog.ResourceLogs, sl plog.ScopeLogs, _ plog.LogRecord) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plogutiltest.NewLogs("AB", "C", "EF"),
			expectTo:   plogutiltest.NewLogs("AB", "D", "EF"),
		},
		{
			name: "move_one",
			moveIf: func(rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeD" && lr.Body().AsString() == "logF"
			},
			from: plogutiltest.NewLogs("AB", "CD", "EF"),
			to:   plog.NewLogs(),
			expectFrom: plogutiltest.NewLogsFromOpts(
				plogutiltest.WithResource('A', plogutiltest.WithScope('C', "EF"), plogutiltest.WithScope('D', "E")),
				plogutiltest.WithResource('B', plogutiltest.WithScope('C', "EF"), plogutiltest.WithScope('D', "EF")),
			),
			expectTo: plogutiltest.NewLogs("A", "D", "F"),
		},
		{
			name: "move_one_from_each_scope",
			moveIf: func(_ plog.ResourceLogs, _ plog.ScopeLogs, lr plog.LogRecord) bool {
				return lr.Body().AsString() == "logE"
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plog.NewLogs(),
			expectFrom: plogutiltest.NewLogs("AB", "CD", "F"),
			expectTo:   plogutiltest.NewLogs("AB", "CD", "E"),
		},
		{
			name: "move_one_from_each_scope_in_one_resource",
			moveIf: func(rl plog.ResourceLogs, _ plog.ScopeLogs, lr plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && lr.Body().AsString() == "logE"
			},
			from: plogutiltest.NewLogs("AB", "CD", "EF"),
			to:   plog.NewLogs(),
			expectFrom: plogutiltest.NewLogsFromOpts(
				plogutiltest.WithResource('A', plogutiltest.WithScope('C', "EF"), plogutiltest.WithScope('D', "EF")),
				plogutiltest.WithResource('B', plogutiltest.WithScope('C', "F"), plogutiltest.WithScope('D', "F")),
			),
			expectTo: plogutiltest.NewLogs("B", "CD", "E"),
		},
		{
			name: "move_some_to_preexisting",
			moveIf: func(_ plog.ResourceLogs, sl plog.ScopeLogs, _ plog.LogRecord) bool {
				return sl.Scope().Name() == "scopeD"
			},
			from:       plogutiltest.NewLogs("AB", "CD", "EF"),
			to:         plogutiltest.NewLogs("1", "2", "3"),
			expectFrom: plogutiltest.NewLogs("AB", "C", "EF"),
			expectTo: plogutiltest.NewLogsFromOpts(
				plogutiltest.WithResource('1', plogutiltest.WithScope('2', "3")),
				plogutiltest.WithResource('A', plogutiltest.WithScope('D', "EF")),
				plogutiltest.WithResource('B', plogutiltest.WithScope('D', "EF")),
			),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			plogutil.MoveRecordsWithContextIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, plogtest.CompareLogs(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, plogtest.CompareLogs(tt.expectTo, tt.to), "to not as expected")
		})
	}
}
