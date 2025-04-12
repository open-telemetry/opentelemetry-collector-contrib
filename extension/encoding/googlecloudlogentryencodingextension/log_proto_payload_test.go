// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"context"
	stdjson "encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestProtoPayload(t *testing.T) {
	tests := []struct {
		scenario string
		config   Config
		expected Log
	}{
		{
			"AsProtobuf",
			Config{
				HandleProtoPayloadAs: HandleAsProtobuf,
			},
			Log{
				Body: map[string]any{
					"@type": "type.googleapis.com/google.cloud.audit.AuditLog",
					"authenticationInfo": map[string]any{
						"principalEmail":               "foo@bar.iam.gserviceaccount.com",
						"principalSubject":             "serviceAccount:foo@bar.iam.gserviceaccount.com",
						"serviceAccountDelegationInfo": []any{map[string]any{"firstPartyPrincipal": map[string]any{"principalEmail": "foo@bar.iam.gserviceaccount.com"}}},
					},
					"authorizationInfo": []any{map[string]any{"resourceAttributes": map[string]any{}}},
					"methodName":        "SearchProjects",
					"request": map[string]any{
						"@type": "type.googleapis.com/google.cloud.resourcemanager.v3.SearchProjectsRequest",
						"query": "state:active AND NOT projectID:sys-*",
					},
					"requestMetadata": map[string]any{
						"callerIp":                "21.128.18.1",
						"callerSuppliedUserAgent": "grpc-go/1.62.2,gzip(gfe)",
						"destinationAttributes":   map[string]any{},
						"requestAttributes":       map[string]any{},
					},
					"resourceName": "projects/project-id",
					"serviceName":  "cloudresourcemanager.googleapis.com",
					"status":       map[string]any{},
				},
			},
		},
		{
			"AsJSON",
			Config{
				HandleProtoPayloadAs: HandleAsJSON,
			},
			Log{
				Body: map[string]any{
					"@type": "type.googleapis.com/google.cloud.audit.AuditLog",
					"authenticationInfo": map[string]any{
						"principalEmail":               "foo@bar.iam.gserviceaccount.com",
						"principalSubject":             "serviceAccount:foo@bar.iam.gserviceaccount.com",
						"serviceAccountDelegationInfo": []any{map[string]any{"firstPartyPrincipal": map[string]any{"principalEmail": "foo@bar.iam.gserviceaccount.com"}}},
					},
					"authorizationInfo": []any{map[string]any{"resourceAttributes": map[string]any{}}},
					"methodName":        "SearchProjects",
					"request": map[string]any{
						"@type": "type.googleapis.com/google.cloud.resourcemanager.v3.SearchProjectsRequest",
						"query": "state:active AND NOT projectID:sys-*",
					},
					"requestMetadata": map[string]any{
						"callerIp":                "21.128.18.1",
						"callerSuppliedUserAgent": "grpc-go/1.62.2,gzip(gfe)",
						"destinationAttributes":   map[string]any{},
						"requestAttributes":       map[string]any{},
					},
					"resourceName": "projects/project-id",
					"serviceName":  "cloudresourcemanager.googleapis.com",
					"status":       map[string]any{},
				},
			},
		},
		{
			"AsText",
			Config{
				HandleProtoPayloadAs: HandleAsText,
			},
			Log{
				Body: "{  \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",  \"status\": {},  \"authenticationInfo\": {    \"principalEmail\": \"foo@bar.iam.gserviceaccount.com\",    \"serviceAccountDelegationInfo\": [      {        \"firstPartyPrincipal\": {          \"principalEmail\": \"foo@bar.iam.gserviceaccount.com\"        }      }    ],    \"principalSubject\": \"serviceAccount:foo@bar.iam.gserviceaccount.com\"  },  \"requestMetadata\": {    \"callerIp\": \"21.128.18.1\",    \"callerSuppliedUserAgent\": \"grpc-go/1.62.2,gzip(gfe)\",    \"requestAttributes\": {},    \"destinationAttributes\": {}  },  \"serviceName\": \"cloudresourcemanager.googleapis.com\",  \"methodName\": \"SearchProjects\",  \"authorizationInfo\": [    {      \"resourceAttributes\": {}    }  ],  \"resourceName\": \"projects/project-id\",  \"request\": {    \"query\": \"state:active AND NOT projectID:sys-*\",    \"@type\": \"type.googleapis.com/google.cloud.resourcemanager.v3.SearchProjectsRequest\"  }}",
			},
		},
	}

	input := "{\"protoPayload\": {  \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",  \"status\": {},  \"authenticationInfo\": {    \"principalEmail\": \"foo@bar.iam.gserviceaccount.com\",    \"serviceAccountDelegationInfo\": [      {        \"firstPartyPrincipal\": {          \"principalEmail\": \"foo@bar.iam.gserviceaccount.com\"        }      }    ],    \"principalSubject\": \"serviceAccount:foo@bar.iam.gserviceaccount.com\"  },  \"requestMetadata\": {    \"callerIp\": \"21.128.18.1\",    \"callerSuppliedUserAgent\": \"grpc-go/1.62.2,gzip(gfe)\",    \"requestAttributes\": {},    \"destinationAttributes\": {}  },  \"serviceName\": \"cloudresourcemanager.googleapis.com\",  \"methodName\": \"SearchProjects\",  \"authorizationInfo\": [    {      \"resourceAttributes\": {}    }  ],  \"resourceName\": \"projects/project-id\",  \"request\": {    \"query\": \"state:active AND NOT projectID:sys-*\",    \"@type\": \"type.googleapis.com/google.cloud.resourcemanager.v3.SearchProjectsRequest\"  }} }"
	for _, tt := range tests {
		fn := func(t *testing.T, want Log) {
			extension := newConfiguredExtension(t, &tt.config)
			defer assert.NoError(t, extension.Shutdown(context.Background()))

			var errs error
			wantRes, wantLr, err := generateLog(t, want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, err := extension.translateLogEntry([]byte(input))
			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			require.NoError(t, errs)
		}
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t, tt.expected)
		})
	}
}

func TestProtoFieldTypes(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		expected Log
	}{
		{
			"String",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",\n    \"serviceName\": \"OpenTelemetry\"\n  }\n}",
			Log{
				Body: map[string]any{
					"@type":       "type.googleapis.com/google.cloud.audit.AuditLog",
					"serviceName": "OpenTelemetry",
				},
			},
		},
		{
			"Boolean",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",\n    \"authorizationInfo\": [\n      {\n        \"granted\": true\n      }\n    ]\n  }\n}",
			Log{
				Body: map[string]any{
					"@type":             "type.googleapis.com/google.cloud.audit.AuditLog",
					"authorizationInfo": []any{map[string]any{"granted": true}},
				},
			},
		},
		{
			"EnumByString",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",\n    \"policyViolationInfo\": {\n      \"orgPolicyViolationInfo\": {\n        \"violationInfo\": [\n          {\n            \"policyType\": \"CUSTOM_CONSTRAINT\"\n          }\n        ]\n      }\n    }\n  }\n}",
			Log{
				Body: map[string]any{
					"@type":               "type.googleapis.com/google.cloud.audit.AuditLog",
					"policyViolationInfo": map[string]any{"orgPolicyViolationInfo": map[string]any{"violationInfo": []any{map[string]any{"policyType": "CUSTOM_CONSTRAINT"}}}},
				},
			},
		},
		{
			"EnumByNumber",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",\n    \"policyViolationInfo\": {\n      \"orgPolicyViolationInfo\": {\n        \"violationInfo\": [\n          {\n            \"policyType\": 3\n          }\n        ]\n      }\n    }\n  }\n}",
			Log{
				Body: map[string]any{
					"@type":               "type.googleapis.com/google.cloud.audit.AuditLog",
					"policyViolationInfo": map[string]any{"orgPolicyViolationInfo": map[string]any{"violationInfo": []any{map[string]any{"policyType": "CUSTOM_CONSTRAINT"}}}},
				},
			},
		},
		{
			"BestEffortAnyType",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.examples/does.not.Exist\",\n    \"noName\": \"Foobar\"\n  }\n}",
			Log{
				Body: map[string]any{
					"noName": "Foobar",
				},
			},
		},
	}

	config := Config{
		HandleProtoPayloadAs: HandleAsProtobuf,
	}
	for _, tt := range tests {
		fn := func(t *testing.T, want Log) {
			extension := newConfiguredExtension(t, &config)
			defer assert.NoError(t, extension.Shutdown(context.Background()))

			var errs error
			wantRes, wantLr, err := generateLog(t, want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, err := extension.translateLogEntry([]byte(tt.input))
			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			require.NoError(t, errs)
		}
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t, tt.expected)
		})
	}
}

func TestProtoErrors(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		error    string
		expected Log
	}{
		{
			"UnknownJSONName",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",\n    \"ServiceName\": 42\n  }\n}",
			"google.cloud.audit.AuditLog has no known field with JSON name ServiceName",
			Log{
				Attributes: map[string]any{
					"gcp.proto_payload": map[string]any{},
				},
				Body: map[string]any{},
			},
		},
		{
			"EnumTypeError",
			"{\n  \"protoPayload\": {\n    \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",\n    \"policyViolationInfo\": {\n      \"orgPolicyViolationInfo\": {\n        \"violationInfo\": [\n          {\n            \"policyType\": {}\n          }\n        ]\n      }\n    }\n  }\n}",
			"wrong type for enum: object",
			Log{
				Body: map[string]any{
					"policyViolationInfo": map[string]any{"orgPolicyViolationInfo": map[string]any{"violationInfo": []any{map[string]any{"policyType": nil}}}},
				},
				Attributes: map[string]any{
					"gcp.proto_payload": map[string]any{
						"policyViolationInfo": map[string]any{"orgPolicyViolationInfo": map[string]any{"violationInfo": []any{map[string]any{"policyType": nil}}}},
					},
				},
			},
		},
	}

	config := Config{
		HandleProtoPayloadAs: HandleAsProtobuf,
	}
	for _, tt := range tests {
		fn := func(t *testing.T, want Log) {
			extension := newConfiguredExtension(t, &config)
			defer assert.NoError(t, extension.Shutdown(context.Background()))

			var errs error
			wantRes, wantLr, err := generateLog(t, want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, translateError := extension.translateLogEntry([]byte(tt.input))
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			assert.ErrorContains(t, translateError, tt.error)
			require.NoError(t, errs)
		}
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t, tt.expected)
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
			j := stdjson.RawMessage{}
			err := j.UnmarshalJSON([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, getTokenType(j))
		})
	}
}
