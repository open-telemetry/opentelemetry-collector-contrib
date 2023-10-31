// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
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
			withoutOptions: errors.New("number of resources doesn't match expected: 2, actual: 1"),
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[testKey2:two]"),
				errors.New("unexpected resource: map[testKey2:one]"),
			),
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareLogsOption{
				IgnoreResourceLogsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resources are out of order: resource "map[testKey1:one]" expected at index 0, found at index 1`),
				errors.New(`resources are out of order: resource "map[testKey2:one]" expected at index 1, found at index 0`),
			),
			withOptions: nil,
		},
		{
			name:           "scope-extra",
			withoutOptions: errors.New(`resource "map[]": number of scopes doesn't match expected: 1, actual: 2`),
		},
		{
			name:           "scope-missing",
			withoutOptions: errors.New(`resource "map[]": number of scopes doesn't match expected: 2, actual: 1`),
		},
		{
			name: "scope-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[]": missing expected scope: one`),
				errors.New(`resource "map[]": unexpected scope: two`),
			),
		},
		{
			name:           "scope-version-mismatch",
			withoutOptions: errors.New(`resource "map[]": scope "one": version doesn't match expected: 1.0, actual: 2.0`),
		},
		{
			name:           "logrecords-missing",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": number of log records doesn't match expected: 1, actual: 0`),
		},
		{
			name: "logrecords-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[type:one]": scope "": missing expected log record: map[testKey2:teststringvalue2 testKey3:teststringvalue3]`),
				errors.New(`resource "map[type:one]": scope "": unexpected log record: map[testKey1:teststringvalue1 testKey2:teststringvalue2]`),
			),
		},
		{
			name:           "logrecords-flag-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": flags doesn't match expected: 1, actual: 2`),
		},
		{
			name:           "logrecords-droppedattributescount-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": dropped attributes count doesn't match expected: 0, actual: 10`),
		},
		{
			name:           "logrecords-timestamp-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000`),
		},
		{
			name:           "logrecords-observedtimestamp-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": observed timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000`),
		},
		{
			name:           "logrecords-severitynumber-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": severity number doesn't match expected: Info, actual: Trace`),
		},
		{
			name:           "logrecords-severitytext-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": severity text doesn't match expected: TEST, actual: OPEN`),
		},
		{
			name:           "logrecords-traceid-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": trace ID doesn't match expected: [139 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130], actual: [123 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130]`),
		},
		{
			name:           "logrecords-spanid-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": span ID doesn't match expected: [12 42 217 36 225 119 22 64], actual: [12 42 217 36 225 119 22 48]`),
		},
		{
			name:           "logrecords-body-mismatch",
			withoutOptions: errors.New(`resource "map[type:one]": scope "": log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": body doesn't match expected: testscopevalue1, actual: testscopevalue2`),
		},
		{
			name: "sort-unordered-log-slice",
			compareOptions: []CompareLogsOption{
				IgnoreLogRecordsOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": scope "collector": log records are out of order: log record "map[testKey3:teststringvalue4]" expected at index 0, found at index 1`),
				errors.New(`resource "map[testKey1:teststringvalue1 testKey2:teststringvalue2]": scope "collector": log records are out of order: log record "map[testKey1:teststringvalue1 testKey2:teststringvalue2]" expected at index 1, found at index 0`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-observed-timestamp",
			compareOptions: []CompareLogsOption{
				IgnoreObservedTimestamp(),
			},
			withoutOptions: errors.New(`resource "map[]": scope "collector": log record "map[]": observed timestamp doesn't match expected: 11651379494838206465, actual: 11651379494838206464`),
			withOptions:    nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadLogs(filepath.Join(dir, "expected.yaml"))
			require.NoError(t, err)

			actual, err := golden.ReadLogs(filepath.Join(dir, "actual.yaml"))
			require.NoError(t, err)

			err = CompareLogs(expected, actual)
			if tc.withoutOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tc.withoutOptions, err.Error())
			}

			if tc.compareOptions == nil {
				return
			}

			err = CompareLogs(expected, actual, tc.compareOptions...)
			if tc.withOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.withOptions.Error())
			}
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
			err: errors.New("attributes don't match expected: map[key1:value1 key2:value2], actual: map[key1:value1]"),
		},
		{
			name: "resource-schema-url-mismatch",
			expected: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.SetSchemaUrl("schema-url")
				return rl
			}(),
			actual: func() plog.ResourceLogs {
				rl := plog.NewResourceLogs()
				rl.SetSchemaUrl("schema-url-2")
				return rl
			}(),
			err: errors.New("schema url doesn't match expected: schema-url, actual: schema-url-2"),
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
			err: errors.New("number of scopes doesn't match expected: 2, actual: 1"),
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
			err: errors.New("name doesn't match expected: scope-name, actual: scope-name-2"),
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
			err: errors.New("version doesn't match expected: scope-version, actual: scope-version-2"),
		},
		{
			name: "scope-attributes-mismatch",
			expected: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().Attributes().PutStr("scope-attr1", "value1")
				sl.Scope().Attributes().PutStr("scope-attr2", "value2")
				return sl
			}(),
			actual: func() plog.ScopeLogs {
				sl := plog.NewScopeLogs()
				sl.Scope().Attributes().PutStr("scope-attr1", "value1")
				sl.Scope().SetDroppedAttributesCount(1)
				return sl
			}(),
			err: multierr.Combine(
				errors.New("attributes don't match expected: map[scope-attr1:value1 scope-attr2:value2], "+
					"actual: map[scope-attr1:value1]"),
				errors.New("dropped attributes count doesn't match expected: 0, actual: 1"),
			),
		},
		{
			name: "scope-schema-url-mismatch",
			expected: func() plog.ScopeLogs {
				rl := plog.NewScopeLogs()
				rl.SetSchemaUrl("schema-url")
				return rl
			}(),
			actual: func() plog.ScopeLogs {
				rl := plog.NewScopeLogs()
				rl.SetSchemaUrl("schema-url-2")
				return rl
			}(),
			err: errors.New("schema url doesn't match expected: schema-url, actual: schema-url-2"),
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
			err: errors.New("number of log records doesn't match expected: 2, actual: 1"),
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
				errors.New(`log records are out of order: log record "map[log-attr1:value1]" expected at index 0, found at index 1`),
				errors.New(`log records are out of order: log record "map[log-attr2:value2]" expected at index 1, found at index 0`),
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
			err: errors.New("attributes don't match expected: map[key1:value1 key2:value1], actual: map[key1:value1 key2:value2]"),
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
			err: errors.New("body doesn't match expected: log-body, actual: log-body-2"),
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
			err: errors.New("timestamp doesn't match expected: 123456789, actual: 987654321"),
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
			err: errors.New("severity number doesn't match expected: Info, actual: Warn"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareLogRecord(test.expected, test.actual))
		})
	}
}
