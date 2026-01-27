// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	ltype "google.golang.org/genproto/googleapis/logging/type"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/auditlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/passthroughnlb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/proxynlb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/vpcflowlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestHandleHTTPRequestField(t *testing.T) {
	tests := []struct {
		name              string
		request           *httpRequest
		expectsAttributes map[string]any
		expectsErr        string
	}{
		{
			name: "valid",
			request: &httpRequest{
				RequestMethod: "POST",
				RequestURL:    "https://www.googleapis.com/logging/v2",
				RequestSize:   "100",
				Status: func() *int64 {
					v := int64(200)
					return &v
				}(),
				ResponseSize: "300",
				UserAgent:    "test",
				RemoteIP:     "127.0.0.2",
				ServerIP:     "127.0.0.3",
				Referer:      "referer",
				Latency:      "10s",
				CacheLookup: func() *bool {
					v := true
					return &v
				}(),
				CacheHit: func() *bool {
					v := true
					return &v
				}(),
				CacheValidatedWithOriginServer: func() *bool {
					v := false
					return &v
				}(),
				CacheFillBytes: "12345",
				Protocol:       "HTTP/1.1",
			},
			expectsAttributes: map[string]any{
				"http.response.size":                  int64(300),
				"http.response.status_code":           int64(200),
				"http.request.method":                 "POST",
				"http.request.size":                   int64(100),
				"url.full":                            "https://www.googleapis.com/logging/v2",
				"url.domain":                          "www.googleapis.com",
				"url.path":                            "/logging/v2",
				gcpCacheFillBytes:                     int64(12345),
				gcpCacheHitField:                      true,
				gcpCacheValidatedWithOriginSeverField: false,
				gcpCacheLookupField:                   true,
				"network.protocol.name":               "http",
				"network.protocol.version":            "1.1",
				refererHeaderField:                    "referer",
				requestServerDurationField:            float64(10),
				"user_agent.original":                 "test",
				"network.peer.address":                "127.0.0.2",
				"server.address":                      "127.0.0.3",
			},
		},
		{
			name: "invalid request size",
			request: &httpRequest{
				RequestSize: "invalid",
			},
			expectsErr: "failed to add request size",
		},
		{
			name: "invalid response size",
			request: &httpRequest{
				ResponseSize: "invalid",
			},
			expectsErr: "failed to add response size",
		},
		{
			name: "invalid cache fill bytes",
			request: &httpRequest{
				CacheFillBytes: "invalid",
			},
			expectsErr: "failed to add cache fill bytes",
		},
		{
			name: "invalid url",
			request: &httpRequest{
				RequestURL: "http://::1]",
			},
			expectsErr: "failed to parse request url",
		},
		{
			name: "invalid protocol",
			request: &httpRequest{
				Protocol: "invalid",
			},
			expectsErr: `expected exactly one "/"`,
		},
		{
			name: "invalid protocol 2",
			request: &httpRequest{
				Protocol: "invalid/",
			},
			expectsErr: "name or version is missing",
		},
		{
			name: "latency without suffix",
			request: &httpRequest{
				Latency: "12",
			},
			expectsErr: "invalid latency format",
		},
		{
			name: "latency not a number",
			request: &httpRequest{
				Latency: "invalids",
			},
			expectsErr: "invalid latency value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			err := handleHTTPRequestField(attr, tt.request)

			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectsAttributes, attr.AsRaw())
		})
	}
}

func TestHandleLogNameField(t *testing.T) {
	tests := []struct {
		name              string
		logName           string
		expectsAttributes map[string]any
		expectsLogType    string
		expectsErr        string
	}{
		{
			name:           "projects",
			logName:        "projects/my-project/logs/log-id",
			expectsLogType: "log-id",
			expectsAttributes: map[string]any{
				gcpProjectField:     "my-project",
				"cloud.resource_id": "log-id",
			},
		},
		{
			name:           "organizations",
			logName:        "organizations/123456/logs/log-id",
			expectsLogType: "log-id",
			expectsAttributes: map[string]any{
				gcpOrganizationField: "123456",
				"cloud.resource_id":  "log-id",
			},
		},
		{
			name:           "billingAccounts",
			logName:        "billingAccounts/BA123/logs/log-id",
			expectsLogType: "log-id",
			expectsAttributes: map[string]any{
				gcpBillingAccountField: "BA123",
				"cloud.resource_id":    "log-id",
			},
		},
		{
			name:           "folders",
			logName:        "folders/456789/logs/log-id",
			expectsLogType: "log-id",
			expectsAttributes: map[string]any{
				gcpFolderField:      "456789",
				"cloud.resource_id": "log-id",
			},
		},
		{
			name:       "unknown prefix",
			logName:    "unknown-prefix",
			expectsErr: "unrecognized log name",
		},
		{
			name:       "empty id",
			logName:    "projects//logs/abc",
			expectsErr: "to have format",
		},
		{
			name:       "empty resource type",
			logName:    "projects/p1/logs/",
			expectsErr: "to have format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			attr := pcommon.NewMap()
			logType, err := handleLogNameField(tt.logName, attr)

			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectsAttributes, attr.AsRaw())
			require.Equal(t, tt.expectsLogType, logType)
		})
	}
}

func TestGetTraceID(t *testing.T) {
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
			expectedErr: "expected trace format to be",
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

			out, err := getTraceID(test.in)
			if err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, out)
			}
		})
	}
}

func TestGetSpanID(t *testing.T) {
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

			out, err := getSpanID(test.in)

			if err != nil {
				require.ErrorContains(t, err, test.expectsErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, out)
			}
		})
	}
}

func TestGetSeverityNumber(t *testing.T) {
	tests := []struct {
		severity string
		expected plog.SeverityNumber
	}{
		{
			severity: ltype.LogSeverity_DEBUG.String(),
			expected: plog.SeverityNumberDebug,
		},
		{
			severity: ltype.LogSeverity_INFO.String(),
			expected: plog.SeverityNumberInfo,
		},
		{
			severity: ltype.LogSeverity_NOTICE.String(),
			expected: plog.SeverityNumberInfo2,
		},
		{
			severity: ltype.LogSeverity_WARNING.String(),
			expected: plog.SeverityNumberWarn,
		},
		{
			severity: ltype.LogSeverity_ERROR.String(),
			expected: plog.SeverityNumberError,
		},
		{
			severity: ltype.LogSeverity_CRITICAL.String(),
			expected: plog.SeverityNumberFatal,
		},
		{
			severity: ltype.LogSeverity_ALERT.String(),
			expected: plog.SeverityNumberFatal2,
		},
		{
			severity: ltype.LogSeverity_EMERGENCY.String(),
			expected: plog.SeverityNumberFatal4,
		},
		{
			severity: ltype.LogSeverity_DEFAULT.String(),
			expected: plog.SeverityNumberUnspecified,
		},
		{
			severity: "unknown",
			expected: plog.SeverityNumberUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			t.Parallel()

			res := getSeverityNumber(tt.severity)
			require.Equal(t, tt.expected, res)
		})
	}
}

func TestHandleLogEntryFields(t *testing.T) {
	// this test will test all common log fields at once
	data, err := os.ReadFile("testdata/log_entry.json")
	require.NoError(t, err)

	var l logEntry
	err = gojson.Unmarshal(data, &l)
	require.NoError(t, err)

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	cfg := *createDefaultConfig().(*Config)

	// add the rest of the log entry fields
	err = handleLogEntryFields(resource.Attributes(), scopeLogs, l, cfg)
	require.NoError(t, err)

	expected, err := golden.ReadLogs("testdata/log_entry_expected.yaml")
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, logs))
}

func TestGetEncodingFormat(t *testing.T) {
	tests := []struct {
		name           string
		logType        string
		expectedFormat string
	}{
		{
			name:           "audit log activity",
			logType:        auditlog.ActivityLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "audit log data access",
			logType:        auditlog.DataAccessLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "audit log system event",
			logType:        auditlog.SystemEventLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "audit log policy",
			logType:        auditlog.PolicyLogNameSuffix,
			expectedFormat: constants.GCPFormatAuditLog,
		},
		{
			name:           "vpc flow log network management",
			logType:        vpcflowlog.NetworkManagementNameSuffix,
			expectedFormat: constants.GCPFormatVPCFlowLog,
		},
		{
			name:           "vpc flow log compute",
			logType:        vpcflowlog.ComputeNameSuffix,
			expectedFormat: constants.GCPFormatVPCFlowLog,
		},
		{
			name:           "proxy nlb log connections",
			logType:        proxynlb.ConnectionsLogNameSuffix,
			expectedFormat: constants.GCPFormatProxyNLBLog,
		},
		{
			name:           "passthrough nlb log connections",
			logType:        passthroughnlb.ConnectionsLogNameSuffix,
			expectedFormat: constants.GCPFormatPassthroughNLBLog,
		},
		{
			name:           "unknown log type",
			logType:        "unknown-log-type",
			expectedFormat: "",
		},
		{
			name:           "empty log type",
			logType:        "",
			expectedFormat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getEncodingFormat(tt.logType)
			require.Equal(t, tt.expectedFormat, result)
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
