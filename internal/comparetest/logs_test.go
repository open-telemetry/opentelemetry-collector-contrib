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

package comparetest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest/golden"
)

func TestCompareLogs(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []LogsCompareOption
		withoutOptions expectation
		withOptions    expectation
	}{
		{
			name: "equal",
		},
		{
			name: "missing",
			withoutOptions: expectation{
				err:    errors.New("amount of ResourceLogs between Logs are not equal expected: 2, actual: 1"),
				reason: "A missing resource should cause a failure",
			},
		},
		{
			name: "resource-attributes-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[testKey2:two]"),
					errors.New("extra resource with attributes: map[testKey2:one]"),
				),
				reason: "A resource with a different set of attributes is a different resource.",
			},
		},
		{
			name: "resource-instrumentation-library-extra",
			withoutOptions: expectation{
				err:    errors.New("number of instrumentation libraries does not match expected: 1, actual: 2"),
				reason: "An extra instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-missing",
			withoutOptions: expectation{
				err:    errors.New("number of instrumentation libraries does not match expected: 2, actual: 1"),
				reason: "An missing instrumentation library should cause a failure.",
			},
		},
		{
			name: "resource-instrumentation-library-name-mismatch",
			withoutOptions: expectation{
				err:    errors.New("instrumentation library Name does not match expected: one, actual: two"),
				reason: "An instrumentation library with a different name is a different library.",
			},
		},
		{
			name: "resource-instrumentation-library-version-mismatch",
			withoutOptions: expectation{
				err:    errors.New("instrumentation library Version does not match expected: 1.0, actual: 2.0"),
				reason: "An instrumentation library with a different version is a different library.",
			},
		},
		{
			name: "logrecords-missing",
			withoutOptions: expectation{
				err:    errors.New("number of log records does not match expected: 1, actual: 0"),
				reason: "A missing log records should cause a failure",
			},
		},
		{
			name: "logrecords-attributes-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log missing expected resource with attributes: map[testKey2:teststringvalue2 testKey3:teststringvalue3]"),
					errors.New("log has extra record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2]"),
				),
				reason: "A log record attributes with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-flag-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record Flags doesn't match expected: 1, actual: 2"),
				),
				reason: "A log record flag with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-droppedattributescount-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record DroppedAttributesCount doesn't match expected: 0, actual: 10"),
				),
				reason: "A log record dropped attributes count with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-timestamp-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record Timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
				),
				reason: "A log record timestamp with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-observedtimestamp-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206464, actual: 11651379494838200000"),
				),
				reason: "A log record observed timestamp with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-severitynumber-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record SeverityNumber doesn't match expected: 9, actual: 1"),
				),
				reason: "A log record severity number with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-severitytext-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record SeverityText doesn't match expected: TEST, actual: OPEN"),
				),
				reason: "A log record severity text with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-traceid-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record TraceID doesn't match expected: [139 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130], actual: [123 32 209 52 158 249 182 214 249 212 209 212 163 172 46 130]"),
				),
				reason: "A log record trace id with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-spanid-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record SpanID doesn't match expected: [12 42 217 36 225 119 22 64], actual: [12 42 217 36 225 119 22 48]"),
				),
				reason: "A log record span id with wrong value should cause a failure",
			},
		},
		{
			name: "logrecords-body-mismatch",
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[testKey1:teststringvalue1 testKey2:teststringvalue2], does not match expected"),
					errors.New("log record Body doesn't match expected: testscopevalue1, actual: testscopevalue2"),
				),
				reason: "A log record body with wrong value should cause a failure",
			},
		},
		{
			name: "sort-unordered-log-slice",
			withoutOptions: expectation{
				err:    nil,
				reason: "A log record body with wrong value should cause a failure",
			},
		},
		{
			name: "ignore-observed-timestamp",
			compareOptions: []LogsCompareOption{
				IgnoreObservedTimestamp(),
			},
			withoutOptions: expectation{
				err: multierr.Combine(
					errors.New("log record with attributes: map[], does not match expected"),
					errors.New("log record ObservedTimestamp doesn't match expected: 11651379494838206465, actual: 11651379494838206464"),
				),
				reason: "A log record body with wrong ObservedTimestamp should cause a failure",
			},
			withOptions: expectation{
				err:    nil,
				reason: "Using IgnoreObservedTimestamp option should mute failure caused by wrong ObservedTimestamp.",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", "logs", tc.name)

			expected, err := golden.ReadLogs(filepath.Join(dir, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadLogs(filepath.Join(dir, "actual.json"))
			require.NoError(t, err)

			err = CompareLogs(expected, actual)
			tc.withoutOptions.validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareLogs(expected, actual, tc.compareOptions...)
			tc.withOptions.validate(t, err)
		})
	}
}
