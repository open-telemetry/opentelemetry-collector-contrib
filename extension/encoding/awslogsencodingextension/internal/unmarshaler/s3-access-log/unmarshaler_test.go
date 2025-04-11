// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3accesslog

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestAddField(t *testing.T) {
	t.Parallel()

	// these tests are checking the conditions
	// that can cause errors, not all possible
	// fields in an access log
	tests := map[string]struct {
		field       int
		value       string
		expectedErr string
	}{
		"invalid_timestamp": {
			field:       fieldIndexTime,
			value:       "invalid",
			expectedErr: "failed to get timestamp of log",
		},
		"valid_timestamp": {
			field: fieldIndexTime,
			value: "[06/Feb/2019:00:00:38 +0000]",
		},
		"invalid_number": {
			field:       fieldIndexObjectSize,
			value:       "invalid",
			expectedErr: "in log line is not a number",
		},
		"valid_number": {
			field: fieldIndexObjectSize,
			value: "3462992",
		},
		"invalid_acl_value": {
			field:       fieldIndexACLRequired,
			value:       "invalid",
			expectedErr: `unknown value "invalid" for field`,
		},
		"valid_acl_value": {
			field: fieldIndexACLRequired,
			value: "Yes",
		},
		"missing_tls_version": {
			field:       fieldIndexTLSVersion,
			value:       "TLSvMissing",
			expectedErr: "missing TLS version",
		},
		"valid_tls_version": {
			field: fieldIndexTLSVersion,
			value: "TLSv1.2",
		},
		"missing_method_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "",
			expectedErr: "has no method",
		},
		"missing_path_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "GET",
			expectedErr: "has no request URI",
		},
		"invalid_request_uri_path": {
			field:       fieldIndexRequestURI,
			value:       "GET " + string([]byte{0x7f}),
			expectedErr: "request uri path is invalid",
		},
		"missing_protocol_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "GET /amzn-s3-demo-bucket1/photos/2019/08/puppy.jpg?x-foo=bar",
			expectedErr: "has no protocol",
		},
		"missing_protocol_name_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "GET /amzn-s3-demo-bucket1/photos/2019/08/puppy.jpg?x-foo=bar /1.1",
			expectedErr: "request uri protocol does not follow expected scheme",
		},
		"missing_protocol_version_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "GET /amzn-s3-demo-bucket1/photos/2019/08/puppy.jpg?x-foo=bar HTTP/",
			expectedErr: "request uri protocol does not follow expected scheme",
		},
		"unexpected_protocol_format_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "GET /amzn-s3-demo-bucket1/photos/2019/08/puppy.jpg?x-foo=bar HTTP1.1",
			expectedErr: "request uri protocol does not follow expected scheme",
		},
		"unexpected_format_request_uri": {
			field:       fieldIndexRequestURI,
			value:       "GET /amzn-s3-demo-bucket1/photos/2019/08/puppy.jpg?x-foo=bar HTTP/1.1 unexpected",
			expectedErr: "does not have expected format",
		},
		"valid_method_request_uri": {
			field: fieldIndexRequestURI,
			value: "GET /amzn-s3-demo-bucket1/photos/2019/08/puppy.jpg?x-foo=bar HTTP/1.1",
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

func TestScanField(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		logLine           string
		expectedValue     string
		expectedRemaining string
		expectsErr        string
	}{
		"empty": {
			logLine:    "",
			expectsErr: io.EOF.Error(),
		},
		"no_quotes": {
			logLine:           "one two three",
			expectedValue:     "one",
			expectedRemaining: "two three",
		},
		"quotes_value_with_no_space": {
			logLine:           `"one" two three`,
			expectedValue:     "one",
			expectedRemaining: "two three",
		},
		"quotes_value_with_spaces": {
			logLine:           `"one two" three`,
			expectedValue:     "one two",
			expectedRemaining: "three",
		},
		"no_end_quote": {
			logLine:    `"one `,
			expectsErr: "has no end quote",
		},
		"two_quoted_values": {
			logLine:           `"one" "two"`,
			expectedValue:     "one",
			expectedRemaining: `"two"`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			value, remaining, err := scanField(test.logLine)
			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expectedValue, value)
			require.Equal(t, test.expectedRemaining, remaining)
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
