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
					errors.New("ResourceLogs with attributes map[testKey1:one] expected at index 0, found a at index 1"),
					errors.New("ResourceLogs with attributes map[testKey2:one] expected at index 1, found a at index 0"),
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
				Err:    errors.New("number of instrumentation libraries does not match expected: 1, actual: 2"),
				Reason: "An extra instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: internal.Expectation{
				Err:    errors.New("number of instrumentation libraries does not match expected: 2, actual: 1"),
				Reason: "An missing instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: internal.Expectation{
				Err:    errors.New("instrumentation library Name does not match expected: one, actual: two"),
				Reason: "An instrumentation library with a different name is a different library.",
			},
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: internal.Expectation{
				Err:    errors.New("instrumentation library Version does not match expected: 1.0, actual: 2.0"),
				Reason: "An instrumentation library with a different version is a different library.",
			},
		},
		{
			name: "logrecords-missing",
			withoutOptions: internal.Expectation{
				Err:    errors.New("number of log records does not match expected: 1, actual: 0"),
				Reason: "A missing log records should cause a failure",
			},
		},
		{
			name: "logrecords-attributes-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
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
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record Flags doesn't match expected: 1, actual: 2"),
				),
				Reason: "A log record flag with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-droppedattributescount-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record DroppedAttributesCount doesn't match expected: 0, actual: 10"),
				),
				Reason: "A log record dropped attributes count with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-timestamp-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record Timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
				),
				Reason: "A log record timestamp with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-observedtimestamp-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
				),
				Reason: "A log record observed timestamp with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-severitynumber-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record SeverityNumber doesn't match expected: 9, actual: 1"),
				),
				Reason: "A log record severity number with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-severitytext-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record SeverityText doesn't match expected: TEST, actual: OPEN"),
				),
				Reason: "A log record severity text with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-traceid-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record TraceID doesn't match expected: [139 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130], actual: [123 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130]"),
				),
				Reason: "A log record trace id with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-spanid-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record SpanID doesn't match expected: [12 42 217 36 225 119 22 64], actual: [12 42 217 36 225 119 22 48]"),
				),
				Reason: "A log record span id with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-body-mismatch",
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
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
					errors.New("LogRecord with attributes map[testKey3:teststringvalue4] expected at index 0, found a at index 1"),
					errors.New("LogRecord with attributes map[testKey1:teststringvalue1 testKey2:teststringvalue2"+
						"] expected at index 1, found a at index 0"),
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
					errors.New("log record with attributes: map[], does not match expected"),
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
