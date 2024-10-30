// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogutil_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/plogutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestMoveResourcesIf(t *testing.T) {
	testCases := []struct {
		name      string
		condition func(plog.ResourceLogs) bool
	}{
		{
			name: "move_none",
			condition: func(plog.ResourceLogs) bool {
				return false
			},
		},
		{
			name: "move_all",
			condition: func(plog.ResourceLogs) bool {
				return true
			},
		},
		{
			name: "move_one",
			condition: func(rl plog.ResourceLogs) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA"
			},
		},
		{
			name: "move_to_preexisting",
			condition: func(rl plog.ResourceLogs) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Load up a fresh copy of the input for each test, since it may be modified in place.
			from, err := golden.ReadLogs(filepath.Join("testdata", "resource", tt.name, "from.yaml"))
			require.NoError(t, err)

			to, err := golden.ReadLogs(filepath.Join("testdata", "resource", tt.name, "to.yaml"))
			require.NoError(t, err)

			fromModifed, err := golden.ReadLogs(filepath.Join("testdata", "resource", tt.name, "from_modified.yaml"))
			require.NoError(t, err)

			toModified, err := golden.ReadLogs(filepath.Join("testdata", "resource", tt.name, "to_modified.yaml"))
			require.NoError(t, err)

			plogutil.MoveResourcesIf(from, to, tt.condition)

			assert.NoError(t, plogtest.CompareLogs(fromModifed, from), "from not modified as expected")
			assert.NoError(t, plogtest.CompareLogs(toModified, to), "to not as expected")
		})
	}
}

func TestMoveRecordsWithContextIf(t *testing.T) {
	testCases := []struct {
		name      string
		condition func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool
	}{
		{
			name: "move_none",
			condition: func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool {
				return false
			},
		},
		{
			name: "move_all",
			condition: func(plog.ResourceLogs, plog.ScopeLogs, plog.LogRecord) bool {
				return true
			},
		},
		{
			name: "move_all_from_one_resource",
			condition: func(rl plog.ResourceLogs, _ plog.ScopeLogs, _ plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
		},
		{
			name: "move_all_from_one_scope",
			condition: func(rl plog.ResourceLogs, sl plog.ScopeLogs, _ plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && sl.Scope().Name() == "scopeA"
			},
		},
		{
			name: "move_all_from_one_scope_in_each_resource",
			condition: func(_ plog.ResourceLogs, sl plog.ScopeLogs, _ plog.LogRecord) bool {
				return sl.Scope().Name() == "scopeB"
			},
		},
		{
			name: "move_one",
			condition: func(rl plog.ResourceLogs, sl plog.ScopeLogs, lr plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA" && sl.Scope().Name() == "scopeB" && lr.Body().AsString() == "logB"
			},
		},
		{
			name: "move_one_from_each_scope",
			condition: func(_ plog.ResourceLogs, _ plog.ScopeLogs, lr plog.LogRecord) bool {
				return lr.Body().AsString() == "logA"
			},
		},
		{
			name: "move_one_from_each_scope_in_one_resource",
			condition: func(rl plog.ResourceLogs, _ plog.ScopeLogs, lr plog.LogRecord) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && lr.Body().AsString() == "logA"
			},
		},
		{
			name: "move_some_to_preexisting",
			condition: func(_ plog.ResourceLogs, sl plog.ScopeLogs, _ plog.LogRecord) bool {
				return sl.Scope().Name() == "scopeB"
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Load up a fresh copy of the input for each test, since it may be modified in place.
			from, err := golden.ReadLogs(filepath.Join("testdata", "record", tt.name, "from.yaml"))
			require.NoError(t, err)

			to, err := golden.ReadLogs(filepath.Join("testdata", "record", tt.name, "to.yaml"))
			require.NoError(t, err)

			fromModifed, err := golden.ReadLogs(filepath.Join("testdata", "record", tt.name, "from_modified.yaml"))
			require.NoError(t, err)

			toModified, err := golden.ReadLogs(filepath.Join("testdata", "record", tt.name, "to_modified.yaml"))
			require.NoError(t, err)

			plogutil.MoveRecordsWithContextIf(from, to, tt.condition)

			assert.NoError(t, plogtest.CompareLogs(fromModifed, from), "from not modified as expected")
			assert.NoError(t, plogtest.CompareLogs(toModified, to), "to not as expected")
		})
	}
}
