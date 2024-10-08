// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func TestMoveLogRecordsWithContextIf(t *testing.T) {
	testCases := []struct {
		name      string
		condition func(plog.LogRecord) bool
	}{
		{
			name: "move_none",
			condition: func(plog.LogRecord) bool {
				return false
			},
		},
		{
			name: "move_all",
			condition: func(plog.LogRecord) bool {
				return true
			},
		},
		{
			name: "move_one",
			condition: func(lr plog.LogRecord) bool {
				return lr.Body().AsString() == "resourceA, scopeB, logB"
			},
		},
		{
			name: "move_all_from_one_scope",
			condition: func(lr plog.LogRecord) bool {
				return strings.HasPrefix(lr.Body().AsString(), "resourceB, scopeA")
			},
		},
		{
			name: "move_all_from_one_resource",
			condition: func(lr plog.LogRecord) bool {
				return strings.HasPrefix(lr.Body().AsString(), "resourceB")
			},
		},
		{
			name: "move_one_from_each_scope",
			condition: func(lr plog.LogRecord) bool {
				return strings.HasSuffix(lr.Body().AsString(), "logA")
			},
		},
		{
			name: "move_all_from_one_scope_in_each_resource",
			condition: func(lr plog.LogRecord) bool {
				return strings.Contains(lr.Body().AsString(), "scopeB")
			},
		},
		{
			name: "move_some_to_preexisting",
			condition: func(lr plog.LogRecord) bool {
				return strings.Contains(lr.Body().AsString(), "scopeB")
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
