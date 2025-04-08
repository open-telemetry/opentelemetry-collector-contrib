// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog

import (
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestAddField(t *testing.T) {
	t.Parallel()

	// these tests are checking the conditions
	// that can cause errors, not all possible
	// fields in an access log
	tests := map[string]struct {
		field       string
		value       string
		expectedErr string
	}{
		"invalid_timestamp": {
			field:       timestamp,
			value:       "invalid",
			expectedErr: "failed to get timestamp of log",
		},
		"valid_timestamp": {
			field: timestamp,
			value: "[06/Feb/2019:00:00:38",
		},
		"invalid_number": {
			field:       attributeAWSS3ObjectSize,
			value:       "invalid",
			expectedErr: "in log line is not a number",
		},
		"valid_number": {
			field: attributeAWSS3ObjectSize,
			value: "3462992",
		},
		"invalid_acl_value": {
			field:       attributeAWSS3AclRequired,
			value:       "invalid",
			expectedErr: `unknown value "invalid" for field`,
		},
		"valid_acl_value": {
			field: attributeAWSS3AclRequired,
			value: "Yes",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := addField(test.field, test.value, &resourceAttributes{}, plog.NewLogRecord())
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRemoveAllQuotes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value      string
		result     string
		expectsErr bool
	}{
		"quoted_value": {
			value:  `"GET"`,
			result: "GET",
		},
		"one_time_quoted_value_start": {
			value:  `"GET`,
			result: "GET",
		},
		"one_time_quoted_value_end": {
			value:  `GET"`,
			result: "GET",
		},
		"no_quotes": {
			value:  "GET",
			result: "GET",
		},
		"data_after_quote": {
			value:      `"Too "Many`,
			expectsErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := removeQuotes(test.value)
			if test.expectsErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.result, result)
		})
	}
}

func TestUnmarshalLogs(t *testing.T) {
	t.Parallel()

	dir := "testdata"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
		expectedErr      string
	}{
		"valid_s3_access_log": {
			// Same access log as in https://docs.aws.amazon.com/AmazonS3/latest/userguide/LogFormat.html
			logFilename:      "valid_s3_access_log.log",
			expectedFilename: "valid_s3_access_log_expected.yaml",
		},
		"unknown_request_uri": {
			// Same valid log, but request uri is unknown
			logFilename:      "unknown_request_uri.log",
			expectedFilename: "unknown_request_uri_expected.yaml",
		},
		"too_few_values": {
			logFilename: "too_few_values.log",
			expectedErr: "values in log line are less than the number of available fields",
		},
		"too_many_values": {
			logFilename: "too_many_values.log",
			expectedErr: "values in log line exceed the number of available fields",
		},
	}

	u := s3AccessLogUnmarshaler{buildInfo: component.BuildInfo{}}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
				return
			}

			require.NoError(t, err)
			expected, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs))
		})
	}
}

func TestParseTime_EqualsTimeParse(t *testing.T) {
	t.Parallel()

	tests := []string{
		"[06/Feb/2019:00:00:38",
		"[16/Jul/2003:17:23:12",
		"[21/Dec/1997:21:37:01",
	}

	// check that the result of time.Parse and
	// parseTime is the same
	for _, timeStr := range tests {
		t.Run(timeStr, func(t *testing.T) {
			t1, err := time.Parse(`[02/Jan/2006:15:04:05 +0000]`, timeStr+" +0000]")
			require.NoError(t, err)

			t2, err := parseTime(timeStr)
			require.NoError(t, err)

			require.Equal(t, t1.UnixNano(), t2.UnixNano())
		})
	}
}

func TestParseTime_Format(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		date       string
		expectsErr string
	}{
		"wrong_format": {
			date:       "wrong-format",
			expectsErr: `timestamp field expects format DD/MM/YYYY:HH:mm:ss`,
		},
		"invalid_day": {
			date:       "[in/Feb/2019:00:00:38",
			expectsErr: `failed to get day from timestamp`,
		},
		"invalid_month": {
			date:       "[06/inv/2019:00:00:38",
			expectsErr: `timestamp contains invalid month`,
		},
		"invalid_year": {
			date:       "[06/Feb/inva:00:00:38",
			expectsErr: `failed to get year from timestamp`,
		},
		"invalid_hour": {
			date:       "[06/Feb/2019:in:00:38",
			expectsErr: `failed to get hour from timestamp`,
		},
		"invalid_minute": {
			date:       "[06/Feb/2019:00:in:38",
			expectsErr: `failed to get minute from timestamp`,
		},
		"invalid_second": {
			date:       "[06/Feb/2019:00:00:in",
			expectsErr: `failed to get second from timestamp`,
		},
		"valid_date": {
			date: "[06/Feb/2019:00:00:38",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseTime(test.date)
			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
