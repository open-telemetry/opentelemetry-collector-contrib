// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func TestMoveLogRecordsWithContextIf(t *testing.T) {
	testCases := []struct {
		name      string
		condition pdatautil.LogRecordFilter
	}{
		{
			name: "move_none",
			condition: func(pcommon.Resource, pcommon.InstrumentationScope, plog.LogRecord) bool {
				return false
			},
		},
		{
			name: "move_all",
			condition: func(pcommon.Resource, pcommon.InstrumentationScope, plog.LogRecord) bool {
				return true
			},
		},
		{
			name: "move_all_from_one_resource",
			condition: func(r pcommon.Resource, _ pcommon.InstrumentationScope, _ plog.LogRecord) bool {
				rname, ok := r.Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
		},
		{
			name: "move_all_from_one_scope",
			condition: func(r pcommon.Resource, s pcommon.InstrumentationScope, _ plog.LogRecord) bool {
				rname, ok := r.Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && s.Name() == "scopeA"
			},
		},
		{
			name: "move_all_from_one_scope_in_each_resource",
			condition: func(_ pcommon.Resource, s pcommon.InstrumentationScope, _ plog.LogRecord) bool {
				return s.Name() == "scopeB"
			},
		},
		{
			name: "move_one",
			condition: func(r pcommon.Resource, s pcommon.InstrumentationScope, lr plog.LogRecord) bool {
				rname, ok := r.Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA" && s.Name() == "scopeB" && lr.Body().AsString() == "logB"
			},
		},
		{
			name: "move_one_from_each_scope",
			condition: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, lr plog.LogRecord) bool {
				return lr.Body().AsString() == "logA"
			},
		},
		{
			name: "move_one_from_each_scope_in_one_resource",
			condition: func(r pcommon.Resource, _ pcommon.InstrumentationScope, lr plog.LogRecord) bool {
				rname, ok := r.Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB" && lr.Body().AsString() == "logA"
			},
		},
		{
			name: "move_some_to_preexisting",
			condition: func(_ pcommon.Resource, s pcommon.InstrumentationScope, _ plog.LogRecord) bool {
				return s.Name() == "scopeB"
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Load up a fresh copy of the input for each test, since it may be modified in place.
			from, err := golden.ReadLogs(filepath.Join("testdata", "logs", tt.name, "from.yaml"))
			require.NoError(t, err)

			to, err := golden.ReadLogs(filepath.Join("testdata", "logs", tt.name, "to.yaml"))
			require.NoError(t, err)

			fromModifed, err := golden.ReadLogs(filepath.Join("testdata", "logs", tt.name, "from_modified.yaml"))
			require.NoError(t, err)

			toModified, err := golden.ReadLogs(filepath.Join("testdata", "logs", tt.name, "to_modified.yaml"))
			require.NoError(t, err)

			pdatautil.MoveLogRecordsWithContextIf(from, to, tt.condition)

			assert.NoError(t, plogtest.CompareLogs(fromModifed, from), "from not modified as expected")
			assert.NoError(t, plogtest.CompareLogs(toModified, to), "to not as expected")
		})
	}
}
