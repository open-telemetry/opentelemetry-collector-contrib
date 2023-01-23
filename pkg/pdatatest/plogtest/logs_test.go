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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

func TestCompareLogs(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareLogsOption
		withoutOptions internal.Expectation
		withOptions    internal.Expectation
	}{
		{
			name: "equal",
		},
		{
			name: "missing",
			withoutOptions: internal.Expectation{
				Err:    errors.New("amount of ResourceLogs between Logs are not equal expected: 2, actual: 1"),
				Reason: "A missing resource should cause a failure",
			},
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[testKey2:two]"),
					errors.New("extra resource with attributes: map[testKey2:one]"),
				),
				Reason: "A resource with a different set of attributes is a different resource.",
			},
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareLogsOption{
				IgnoreResourceLogsOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[testKey1:one] expected at index 0, found at index 1"),
					errors.New("ResourceLogs with attributes map[testKey2:one] expected at index 1, found at index 0"),
				),
				Reason: "Resource order mismatch will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "Ignored resource order mismatch should not cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-extra",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[] does not match expected"),
					errors.New("number of scope logs does not match expected: 1, actual: 2"),
				),
				Reason: "An extra instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[] does not match expected"),
					errors.New("number of scope logs does not match expected: 2, actual: 1"),
				),
				Reason: "An missing instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[] does not match expected"),
					errors.New("missing ScopeLogs with scope name: one"),
					errors.New("unexpected ScopeLogs with scope name: two"),
				),
				Reason: "An instrumentation library with a different name is a different library.",
			},
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[] does not match expected"),
					errors.New(`ScopeLogs with scope name "one" do not match expected`),
					errors.New("scope version does not match expected: 1.0, actual: 2.0"),
				),
				Reason: "An instrumentation library with a different version is a different library.",
			},
		},
		{
			name: "logrecords-missing",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("number of log records does not match expected: 1, actual: 0"),
				),
				Reason: "A missing log records should cause a failure",
			},
		},
		{
			name: "logrecords-attributes-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log missing expected resource with attributes: map[testKey2:teststringvalue2 testKey3:teststringvalue3]"),
					errors.New("log has extra record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2]"),
				),
				Reason: "A log record attributes with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-flag-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record Flags doesn't match expected: 1, actual: 2"),
				),
				Reason: "A log record flag with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-droppedattributescount-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record DroppedAttributesCount doesn't match expected: 0, actual: 10"),
				),
				Reason: "A log record dropped attributes count with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-timestamp-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record Timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
				),
				Reason: "A log record timestamp with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-observedtimestamp-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
				),
				Reason: "A log record observed timestamp with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-severitynumber-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record SeverityNumber doesn't match expected: Info, actual: Trace"),
				),
				Reason: "A log record severity number with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-severitytext-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record SeverityText doesn't match expected: TEST, actual: OPEN"),
				),
				Reason: "A log record severity text with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-traceid-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record TraceID doesn't match expected: [139 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130], actual: [123 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130]"),
				),
				Reason: "A log record trace id with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-spanid-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record SpanID doesn't match expected: [12 42 217 36 225 119 22 64], actual: [12 42 217 36 225 119 22 48]"),
				),
				Reason: "A log record span id with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-body-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[type:one] does not match expected"),
					errors.New(`ScopeLogs with scope name "" do not match expected`),
					errors.New("log record with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New("log record Body doesn't match expected: testscopevalue1, actual: testscopevalue2"),
				),
				Reason: "A log record body with wrong value should cause a failure",
			},
		},
		{
			name: "sort-unordered-log-slice",
			compareOptions: []CompareLogsOption{
				IgnoreLogRecordsOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2] does not match expected"),
					errors.New(`ScopeLogs with scope name "collector" do not match expected`),
					errors.New("LogRecord with attributes map[testKey3:teststringvalue4] expected at index 0, found at index 1"),
					errors.New("LogRecord with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2"+
						"] expected at index 1, found at index 0"),
				),
				Reason: "A log record records with different order should cause a failure",
			},
			withOptions: internal.Expectation{
				Err: nil,
				Reason: "A log record records with different order should not cause a failure if" +
					" IgnoreLogRecordsOrder is applied",
			},
		},
		{
			name: "ignore-observed-timestamp",
			compareOptions: []CompareLogsOption{
				IgnoreObservedTimestamp(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceLogs with attributes map[] does not match expected"),
					errors.New(`ScopeLogs with scope name "collector" do not match expected`),
					errors.New("log record with attributes map[] does not match expected"),
					errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206465, actual: 11651379494838206464"),
				),
				Reason: "A log record body with wrong ObservedTimestamp should cause a failure",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "Using IgnoreObservedTimestamp option should mute failure caused by wrong ObservedTimestamp.",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadLogs(filepath.Join(dir, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadLogs(filepath.Join(dir, "actual.json"))
			require.NoError(t, err)

			err = CompareLogs(expected, actual)
			tc.withoutOptions.Validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareLogs(expected, actual, tc.compareOptions...)
			tc.withOptions.Validate(t, err)
		})
	}
}

func TestCompareResourceLogs(t *testing.T) {
	tests := []struct {
		name     string
		expected plog.ResourceLogs
		actual   plog.ResourceLogs
		err      internal.Expectation
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
			err: internal.Expectation{
				Err:    errors.New("resource attributes do not match expected: map[key1:value1 key2:value2], actual: map[key1:value1]"),
				Reason: "A log record records with different order should cause a failure",
			},
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
			err: internal.Expectation{
				Err:    errors.New("number of scope logs does not match expected: 2, actual: 1"),
				Reason: "Scope logs with different number should cause a failure",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.err.Validate(t, CompareResourceLogs(test.expected, test.actual))
		})
	}
}

func TestCompareScopeLogs(t *testing.T) {
	tests := []struct {
		name     string
		expected plog.ScopeLogs
		actual   plog.ScopeLogs
		err      internal.Expectation
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
			err: internal.Expectation{
				Err: errors.New("scope name does not match expected: scope-name, actual: scope-name-2"),
			},
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
			err: internal.Expectation{
				Err:    errors.New("scope version does not match expected: scope-version, actual: scope-version-2"),
				Reason: "Scope logs with different versions should cause a failure",
			},
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
			err: internal.Expectation{
				Err:    errors.New("number of log records does not match expected: 2, actual: 1"),
				Reason: "Log records with different number should cause a failure",
			},
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
			err: internal.Expectation{
				Err: multierr.Combine(
					errors.New("LogRecord with attributes map[log-attr1:value1] expected at index 0, found at index 1"),
					errors.New("LogRecord with attributes map[log-attr2:value2] expected at index 1, found at index 0"),
				),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.err.Validate(t, CompareScopeLogs(test.expected, test.actual))
		})
	}
}

func TestCompareLogRecord(t *testing.T) {
	tests := []struct {
		name     string
		expected plog.LogRecord
		actual   plog.LogRecord
		err      internal.Expectation
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
			err: internal.Expectation{
				Err: errors.New("log record attributes do not match expected: map[key1:value1 key2:value1], " +
					"actual: map[key1:value1 key2:value2]"),
				Reason: "Log records with different attributes should cause a failure",
			},
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
			err: internal.Expectation{
				Err:    errors.New("log record Body doesn't match expected: log-body, actual: log-body-2"),
				Reason: "Log records with different body should cause a failure",
			},
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
			err: internal.Expectation{
				Err:    errors.New("log record Timestamp doesn't match expected: 123456789, actual: 987654321"),
				Reason: "Log records with different timestamp should cause a failure",
			},
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
			err: internal.Expectation{
				Err:    errors.New("log record SeverityNumber doesn't match expected: Info, actual: Warn"),
				Reason: "Log records with different severity number should cause a failure",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.err.Validate(t, CompareLogRecord(test.expected, test.actual))
		})
	}
}
