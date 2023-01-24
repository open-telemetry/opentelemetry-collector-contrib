// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plogtest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
)

func TestCompareLogs(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareLogsOption
		withoutOptions error
		withOptions    error
	}{
		{
			name: "equal",
		},
		{
			name:           "missing",
			withoutOptions: errors.New("amount of ResourceLogs between Logs are not equal expected: 2, actual: 1"),
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource with attributes: map[testKey2:two]"),
				errors.New("extra resource with attributes: map[testKey2:one]"),
			),
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareLogsOption{
				IgnoreResourceLogsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[testKey1:one] expected at index 0, found at index 1"),
				errors.New("ResourceLogs with attributes map[testKey2:one] expected at index 1, found at index 0"),
			),
			withOptions: nil,
		},
		{
			name: "resource-instrumentation-library-extra",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[] does not match expected"),
				errors.New("number of scope logs does not match expected: 1, actual: 2"),
			),
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[] does not match expected"),
				errors.New("number of scope logs does not match expected: 2, actual: 1"),
			),
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[] does not match expected"),
				errors.New("missing ScopeLogs with scope name: one"),
				errors.New("unexpected ScopeLogs with scope name: two"),
			),
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[] does not match expected"),
				errors.New(`ScopeLogs with scope name "one" do not match expected`),
				errors.New("scope version does not match expected: 1.0, actual: 2.0"),
			),
		},
		{
			name: "logrecords-missing",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("number of log records does not match expected: 1, actual: 0"),
			),
		},
		{
			name: "logrecords-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log missing expected resource with attributes: map[testKey2:teststringvalue2 testKey3:teststringvalue3]"),
				errors.New("log has extra record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2]"),
			),
		},
		{
			name: "logrecords-flag-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record Flags doesn't match expected: 1, actual: 2"),
			),
		},
		{
			name: "logrecords-droppedattributescount-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record DroppedAttributesCount doesn't match expected: 0, actual: 10"),
			),
		},
		{
			name: "logrecords-timestamp-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record Timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
			),
		},
		{
			name: "logrecords-observedtimestamp-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
			),
		},
		{
			name: "logrecords-severitynumber-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record SeverityNumber doesn't match expected: Info, actual: Trace"),
			),
		},
		{
			name: "logrecords-severitytext-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record SeverityText doesn't match expected: TEST, actual: OPEN"),
			),
		},
		{
			name: "logrecords-traceid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record TraceID doesn't match expected: [139 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130], actual: [123 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130]"),
			),
		},
		{
			name: "logrecords-spanid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record SpanID doesn't match expected: [12 42 217 36 225 119 22 64], actual: [12 42 217 36 225 119 22 48]"),
			),
		},
		{
			name: "logrecords-body-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
				errors.New(`ScopeLogs with scope name "" do not match expected`),
				errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New("log record Body doesn't match expected: testscopevalue1, actual: testscopevalue2"),
			),
		},
		{
			name: "sort-unordered-log-slice",
			compareOptions: []CompareLogsOption{
				IgnoreLogRecordsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
				errors.New(`ScopeLogs with scope name "collector" do not match expected`),
				errors.New("LogRecord with attributes map[testKey3:teststringvalue4] expected at index 0, found at index 1"),
				errors.New("LogRecord with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2"+
					"] expected at index 1, found at index 0"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-observed-timestamp",
			compareOptions: []CompareLogsOption{
				IgnoreObservedTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New("ResourceLogs with attributes map[] does not match expected"),
				errors.New(`ScopeLogs with scope name "collector" do not match expected`),
				errors.New("log record with attributes map[] does not match expected"),
				errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206465, actual: 11651379494838206464"),
			),
			withOptions: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadLogs(filepath.Join(dir, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadLogs(filepath.Join(dir, "actual.json"))
			require.NoError(t, err)

			assert.Equal(t, tc.withoutOptions, CompareLogs(expected, actual))

			if tc.compareOptions == nil {
				return
			}

			assert.Equal(t, tc.withOptions, CompareLogs(expected, actual, tc.compareOptions...))
		})
	}
}

func TestCompareResourceLogs(t *testing.T) {
	tests := []struct {
		name     string
		expected plog.ResourceLogs
		actual   plog.ResourceLogs
		err      error
	}{
		{
			name: "equal",
			expected: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr1", "value1")
				l.Body().SetStr("log-body")
				return rl
			}(),
			actual: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr1", "value1")
				l.Body().SetStr("log-body")
				return rl
			}(),
		},
		{
			name: "resource-attributes-mismatch",
			expected: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.Resource().Attributes().PutStr("key1", "value1")
				rl.Resource().Attributes().PutStr("key2", "value2")
				return rl
			}(),
			actual: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.Resource().Attributes().PutStr("key1", "value1")
				return rl
			}(),
			err: errors.New("resource attributes do not match expected: map[key1:value1 key2:value2], actual: map[key1:value1]"),
		},
		{
			name: "scope-logs-number-mismatch",
			expected: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.ScopeLogs().AppendEmpty()
				rl.ScopeLogs().AppendEmpty()
				return rl
			}(),
			actual: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.ScopeLogs().AppendEmpty()
				return rl
			}(),
			err: errors.New("number of scope logs does not match expected: 2, actual: 1"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareResourceLogs(test.expected, test.actual))
		})
	}
}

func TestCompareScopeLogs(t *testing.T) {
	tests := []struct {
		name     string
		expected plog.ScopeLogs
		actual   plog.ScopeLogs
		err      error
	}{
		{
			name: "equal",
			expected: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().SetName("scope-name")
				l := sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr1", "value1")
				l.Body().SetStr("log-body")
				return sl
			}(),
			actual: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().SetName("scope-name")
				l := sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr1", "value1")
				l.Body().SetStr("log-body")
				return sl
			}(),
		},
		{
			name: "scope-name-mismatch",
			expected: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().SetName("scope-name")
				return sl
			}(),
			actual: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().SetName("scope-name-2")
				return sl
			}(),
			err: errors.New("scope name does not match expected: scope-name, actual: scope-name-2"),
		},
		{
			name: "scope-version-mismatch",
			expected: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().SetVersion("scope-version")
				return sl
			}(),
			actual: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().SetVersion("scope-version-2")
				return sl
			}(),
			err: errors.New("scope version does not match expected: scope-version, actual: scope-version-2"),
		},
		{
			name: "log-records-number-mismatch",
			expected: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.LogRecords().AppendEmpty()
				sl.LogRecords().AppendEmpty()
				return sl
			}(),
			actual: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.LogRecords().AppendEmpty()
				return sl
			}(),
			err: errors.New("number of log records does not match expected: 2, actual: 1"),
		},
		{
			name: "log-records-order-mismatch",
			expected: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				l := sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr1", "value1")
				l.Body().SetStr("log-body")
				l = sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr2", "value2")
				l.Body().SetStr("log-body-2")
				return sl
			}(),
			actual: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				l := sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr2", "value2")
				l.Body().SetStr("log-body-2")
				l = sl.LogRecords().AppendEmpty()
				l.Attributes().PutStr("log-attr1", "value1")
				l.Body().SetStr("log-body")
				return sl
			}(),
			err: multierr.Combine(
				errors.New("LogRecord with attributes map[log-attr1:value1] expected at index 0, found at index 1"),
				errors.New("LogRecord with attributes map[log-attr2:value2] expected at index 1, found at index 0"),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareScopeLogs(test.expected, test.actual))
		})
	}
}

func TestCompareLogRecord(t *testing.T) {
	tests := []struct {
		name     string
		expected plog.LogRecord
		actual   plog.LogRecord
		err      error
	}{
		{
			name: "equal",
			expected: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr("key1", "value1")
				lr.Attributes().PutStr("key2", "value1")
				lr.Body().SetStr("log-body")
				lr.SetTimestamp(pcommon.Timestamp(123456789))
				lr.SetSeverityNumber(plog.SeverityNumberInfo)
				lr.SetSeverityText("INFO")
				return lr
			}(),
			actual: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr("key1", "value1")
				lr.Attributes().PutStr("key2", "value1")
				lr.Body().SetStr("log-body")
				lr.SetTimestamp(pcommon.Timestamp(123456789))
				lr.SetSeverityNumber(plog.SeverityNumberInfo)
				lr.SetSeverityText("INFO")
				return lr
			}(),
		},
		{
			name: "attributes-mismatch",
			expected: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr("key1", "value1")
				lr.Attributes().PutStr("key2", "value1")
				return lr
			}(),
			actual: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr("key1", "value1")
				lr.Attributes().PutStr("key2", "value2")
				return lr
			}(),
			err: errors.New("log record attributes do not match expected: map[key1:value1 key2:value1], " +
				"actual: map[key1:value1 key2:value2]"),
		},
		{
			name: "body-mismatch",
			expected: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetStr("log-body")
				return lr
			}(),
			actual: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetStr("log-body-2")
				return lr
			}(),
			err: errors.New("log record Body doesn't match expected: log-body, actual: log-body-2"),
		},
		{
			name: "timestamp-mismatch",
			expected: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetTimestamp(pcommon.Timestamp(123456789))
				return lr
			}(),
			actual: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetTimestamp(pcommon.Timestamp(987654321))
				return lr
			}(),
			err: errors.New("log record Timestamp doesn't match expected: 123456789, actual: 987654321"),
		},
		{
			name: "severity-number-mismatch",
			expected: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetSeverityNumber(plog.SeverityNumberInfo)
				return lr
			}(),
			actual: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetSeverityNumber(plog.SeverityNumberWarn)
				return lr
			}(),
			err: errors.New("log record SeverityNumber doesn't match expected: Info, actual: Warn"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareLogRecord(test.expected, test.actual))
		})
	}
}
