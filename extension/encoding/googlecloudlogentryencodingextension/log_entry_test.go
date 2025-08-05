// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestCloudLoggingTraceToTraceIDBytes(t *testing.T) {
	for _, test := range []struct {
		scenario,
		in string
		out         [16]byte
		expectedErr string
	}{
		{
			scenario: "valid_trace",
			in:       "projects/my-gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e09",
			out:      [16]uint8{0x1d, 0xbe, 0x31, 0x7e, 0xb7, 0x3e, 0xb6, 0xe3, 0xbb, 0xb5, 0x1a, 0x2b, 0xc3, 0xa4, 0x1e, 0x9},
		},
		{
			scenario:    "invalid_trace_format",
			in:          "1dbe317eb73eb6e3bbb51a2bc3a41e09",
			expectedErr: "invalid trace format",
		},
		{
			scenario:    "invalid_hex_trace",
			in:          "projects/my-gcp-project/traces/xyze317eb73eb6e3bbb51a2bc3a41e09",
			expectedErr: "failed to decode trace id to hexadecimal string",
		},
		{
			scenario:    "invalid_trace_length",
			in:          "projects/my-gcp-project/traces/1dbe317eb73eb6e3bbb51a2bc3a41e",
			expectedErr: "expected trace ID hex length to be 16",
		},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			t.Parallel()

			out, err := cloudLoggingTraceToTraceIDBytes(test.in)
			if err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, out)
			}
		})
	}
}

func TestSpanIDStrToSpanIDBytes(t *testing.T) {
	for _, test := range []struct {
		scenario,
		in string
		out        [8]byte
		expectsErr string
	}{
		{
			scenario: "valid_span",
			in:       "3e3a5741b18f0710",
			out:      [8]uint8{0x3e, 0x3a, 0x57, 0x41, 0xb1, 0x8f, 0x7, 0x10},
		},
		{
			scenario:   "invalid_hex_span",
			in:         "123/123",
			expectsErr: "failed to decode span id to hexadecimal string",
		},
		{
			scenario:   "invalid_span_length",
			in:         "3e3a5741b18f0710ab",
			expectsErr: "expected span ID hex length to be 8",
		},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			t.Parallel()

			out, err := spanIDStrToSpanIDBytes(test.in)

			if err != nil {
				require.ErrorContains(t, err, test.expectsErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, out)
			}
		})
	}
}

func TestCloudLoggingSeverityToNumber(t *testing.T) {
	tests := []struct {
		severity string
		expected plog.SeverityNumber
	}{
		{
			severity: "DEBUG",
			expected: plog.SeverityNumberDebug,
		},
		{
			severity: "INFO",
			expected: plog.SeverityNumberInfo,
		},
		{
			severity: "NOTICE",
			expected: plog.SeverityNumberInfo2,
		},
		{
			severity: "WARNING",
			expected: plog.SeverityNumberWarn,
		},
		{
			severity: "ERROR",
			expected: plog.SeverityNumberError,
		},
		{
			severity: "CRITICAL",
			expected: plog.SeverityNumberFatal,
		},
		{
			severity: "ALERT",
			expected: plog.SeverityNumberFatal2,
		},
		{
			severity: "EMERGENCY",
			expected: plog.SeverityNumberFatal4,
		},
		{
			severity: "DEFAULT",
			expected: plog.SeverityNumberUnspecified,
		},
		{
			severity: "UNKNOWN",
			expected: plog.SeverityNumberUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			t.Parallel()

			res := cloudLoggingSeverityToNumber(tt.severity)
			require.Equal(t, tt.expected, res)
		})
	}
}

func TestHandleJSONPayload(t *testing.T) {
	tests := []struct {
		name        string
		jsonPayload gojson.RawMessage
		expectsBody any
		cfg         Config
		expectsErr  string
	}{
		{
			name: "valid_json_payload_as_json",
			cfg: Config{
				HandleJSONPayloadAs: HandleAsJSON,
			},
			jsonPayload: []byte(`{"test": "test"}`),
			expectsBody: map[string]any{
				"test": "test",
			},
		},
		{
			name: "invalid_json_payload_as_json",
			cfg: Config{
				HandleJSONPayloadAs: HandleAsJSON,
			},
			jsonPayload: []byte("invalid"),
			expectsErr:  "failed to unmarshal JSON payload",
		},
		{
			name: "valid_json_payload_as_text",
			cfg: Config{
				HandleJSONPayloadAs: HandleAsText,
			},
			jsonPayload: []byte(`{"test": "test"}`),
			expectsBody: `{"test": "test"}`,
		},
		{
			name: "invalid_json_payload_unknown",
			cfg: Config{
				HandleJSONPayloadAs: "unknown",
			},
			jsonPayload: []byte("does-not-matter"),
			expectsErr:  "unrecognized JSON payload type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lr := plog.NewLogRecord()

			err := handleJSONPayloadField(lr, tt.jsonPayload, tt.cfg)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, lr.Body().AsRaw(), tt.expectsBody)
		})
	}
}

func TestHandleProtoPayload(t *testing.T) {
	tests := []struct {
		name         string
		protoPayload gojson.RawMessage
		expectsBody  any
		cfg          Config
		expectsErr   string
	}{
		{
			name:         "valid_proto_payload_as_proto",
			protoPayload: []byte(`{"@type":"test"}`),
			cfg: Config{
				HandleProtoPayloadAs: HandleAsProtobuf,
			},
			expectsBody: map[string]any{},
		},
		{
			name:         "invalid_proto_payload_as_proto",
			protoPayload: []byte("invalid"),
			cfg: Config{
				HandleProtoPayloadAs: HandleAsProtobuf,
			},
			expectsErr: "failed to set body from proto payload",
		},
		{
			name:         "valid_proto_payload_as_json",
			protoPayload: []byte(`{"@type":"test"}`),
			cfg: Config{
				HandleProtoPayloadAs: HandleAsJSON,
			},
			expectsBody: map[string]any{
				"@type": "test",
			},
		},
		{
			name:         "invalid_proto_payload_as_json",
			protoPayload: []byte("invalid"),
			cfg: Config{
				HandleProtoPayloadAs: HandleAsJSON,
			},
			expectsErr: "failed to unmarshal JSON payload",
		},
		{
			name:         "valid_proto_payload_as_text",
			protoPayload: []byte(`{"@type":"test"}`),
			cfg: Config{
				HandleProtoPayloadAs: HandleAsText,
			},
			expectsBody: `{"@type":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lr := plog.NewLogRecord()

			err := handleProtoPayloadField(lr, tt.protoPayload, tt.cfg)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, lr.Body().AsRaw(), tt.expectsBody)
		})
	}
}
