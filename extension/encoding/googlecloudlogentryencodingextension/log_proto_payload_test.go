// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestProtoPayload(t *testing.T) {
	tests := []struct {
		scenario string
		config   Config
		wantFile string
	}{
		{
			scenario: "AsProtobuf",
			config: Config{
				HandleProtoPayloadAs: HandleAsProtobuf,
			},
			wantFile: "testdata/proto_payload/as_protofobuf_expected.yaml",
		},
		{
			scenario: "AsJSON",
			config: Config{
				HandleProtoPayloadAs: HandleAsJSON,
			},
			wantFile: "testdata/proto_payload/as_json_expected.yaml",
		},
		{
			scenario: "AsText",
			config: Config{
				HandleProtoPayloadAs: HandleAsText,
			},
			wantFile: "testdata/proto_payload/as_text_expected.yaml",
		},
	}

	input, err := os.ReadFile("testdata/proto_payload/proto_payload.json")
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			t.Parallel()

			extension := newTestExtension(t, tt.config)

			wantRes, err := golden.ReadLogs(tt.wantFile)
			require.NoError(t, err)

			gotRes, err := extension.UnmarshalLogs(input)
			require.NoError(t, err)

			require.NoError(t, plogtest.CompareLogs(wantRes, gotRes))
		})
	}
}

func TestProtoFieldTypes(t *testing.T) {
	tests := []struct {
		scenario     string
		input        []byte
		expectedBody any
	}{
		{
			scenario: "String",
			input: []byte(`{
  "protoPayload": {
    "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
    "serviceName": "OpenTelemetry"
  }
}`),
			expectedBody: map[string]any{
				"@type":       "type.googleapis.com/google.cloud.audit.AuditLog",
				"serviceName": "OpenTelemetry",
			},
		},
		{
			scenario: "Boolean",
			input: []byte(`{
  "protoPayload": {
	"@type": "type.googleapis.com/google.cloud.audit.AuditLog",
	"authorizationInfo": [
	  {
		"granted": true
	  }
	]
  }
}`),
			expectedBody: map[string]any{
				"@type":             "type.googleapis.com/google.cloud.audit.AuditLog",
				"authorizationInfo": []any{map[string]any{"granted": true}},
			},
		},

		{
			scenario: "EnumByString",
			input: []byte(`{
  "protoPayload": {
	"@type": "type.googleapis.com/google.cloud.audit.AuditLog",
	"policyViolationInfo": {
	  "orgPolicyViolationInfo": {
		"violationInfo": [
		  {
			"policyType": "CUSTOM_CONSTRAINT"
		  }
		]
	  }
	}
  }
}`),
			expectedBody: map[string]any{
				"@type":               "type.googleapis.com/google.cloud.audit.AuditLog",
				"policyViolationInfo": map[string]any{"orgPolicyViolationInfo": map[string]any{"violationInfo": []any{map[string]any{"policyType": "CUSTOM_CONSTRAINT"}}}},
			},
		},
		{
			scenario: "EnumByNumber",
			input: []byte(`{
  "protoPayload": {
	"@type": "type.googleapis.com/google.cloud.audit.AuditLog",
	"policyViolationInfo": {
	  "orgPolicyViolationInfo": {
		"violationInfo": [
		  {
			"policyType": 3
		  }
		]
	  }
	}
  }
}`),
			expectedBody: map[string]any{
				"@type":               "type.googleapis.com/google.cloud.audit.AuditLog",
				"policyViolationInfo": map[string]any{"orgPolicyViolationInfo": map[string]any{"violationInfo": []any{map[string]any{"policyType": "CUSTOM_CONSTRAINT"}}}},
			},
		},
		{
			scenario: "BestEffortAnyType",
			input: []byte(`{
  "protoPayload": {
	"@type": "type.examples/does.not.Exist",
	"noName": "Foobar"
  }
}`),
			expectedBody: map[string]any{
				"noName": "Foobar",
			},
		},
	}

	config := Config{
		HandleProtoPayloadAs: HandleAsProtobuf,
	}
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			t.Parallel()

			extension := newTestExtension(t, config)

			gotRes, err := extension.UnmarshalLogs(tt.input)
			require.NoError(t, err)

			require.Equal(t, 1, gotRes.LogRecordCount())

			lr := gotRes.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			require.Equal(t, tt.expectedBody, lr.Body().AsRaw())
		})
	}
}

func TestProtoErrors(t *testing.T) {
	tests := []struct {
		scenario   string
		input      []byte
		expectsErr string
	}{
		{
			scenario: "UnknownJSONName",
			input: []byte(`{
  "protoPayload": {
    "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
    "ServiceName": 42
  }
}`),
			expectsErr: "google.cloud.audit.AuditLog has no known field with JSON name ServiceName",
		},
		{
			scenario: "EnumTypeError",
			input: []byte(`{
  "protoPayload": {
    "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
    "policyViolationInfo": {
      "orgPolicyViolationInfo": {
        "violationInfo": [
          {
            "policyType": {}
          }
        ]
      }
    }
  }
}`),
			expectsErr: "wrong type for enum: object",
		},
	}

	config := Config{
		HandleProtoPayloadAs: HandleAsProtobuf,
	}
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			t.Parallel()

			extension := newTestExtension(t, config)

			_, err := extension.translateLogEntry(tt.input)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.expectsErr)
		})
	}
}

func TestGetTokenType(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		expected string
	}{
		{
			"Object",
			"{}",
			"object",
		},
		{
			"Object",
			"[]",
			"array",
		},
		{
			"Number",
			"1.1",
			"number",
		},
		{
			"Boolean",
			"true",
			"bool",
		},
		{
			"String",
			"\"\"",
			"string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			t.Parallel()

			j := gojson.RawMessage{}
			err := j.UnmarshalJSON([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, getTokenType(j))
		})
	}
}
