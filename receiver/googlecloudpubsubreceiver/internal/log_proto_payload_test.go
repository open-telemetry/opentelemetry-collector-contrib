// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

func TestTranslateProtoPayloadLogEntry(t *testing.T) {

	tests := []struct {
		scenario    string
		input       string
		wantAsProto Log
	}{
		{
			"ProtoPayload",
			"{\"protoPayload\": {  \"@type\": \"type.googleapis.com/google.cloud.audit.AuditLog\",  \"status\": {},  \"authenticationInfo\": {    \"principalEmail\": \"foo@bar.iam.gserviceaccount.com\",    \"serviceAccountDelegationInfo\": [      {        \"firstPartyPrincipal\": {          \"principalEmail\": \"foo@bar.iam.gserviceaccount.com\"        }      }    ],    \"principalSubject\": \"serviceAccount:foo@bar.iam.gserviceaccount.com\"  },  \"requestMetadata\": {    \"callerIp\": \"21.128.18.1\",    \"callerSuppliedUserAgent\": \"grpc-go/1.62.2,gzip(gfe)\",    \"requestAttributes\": {},    \"destinationAttributes\": {}  },  \"serviceName\": \"cloudresourcemanager.googleapis.com\",  \"methodName\": \"SearchProjects\",  \"authorizationInfo\": [    {      \"resourceAttributes\": {}    }  ],  \"resourceName\": \"projects/project-id\",  \"request\": {    \"query\": \"state:active AND NOT projectID:sys-*\",    \"@type\": \"type.googleapis.com/google.cloud.resourcemanager.v3.SearchProjectsRequest\"  }} }",
			Log{
				Body: map[string]any{
					"@type": string("type.googleapis.com/google.cloud.audit.AuditLog"),
					"authenticationInfo": map[string]any{
						"principalEmail":               string("foo@bar.iam.gserviceaccount.com"),
						"principalSubject":             string("serviceAccount:foo@bar.iam.gserviceaccount.com"),
						"serviceAccountDelegationInfo": []any{map[string]any{"firstPartyPrincipal": map[string]any{"principalEmail": "foo@bar.iam.gserviceaccount.com"}}},
					},
					"authorizationInfo": []any{map[string]any{"resourceAttributes": map[string]any{}}},
					"methodName":        string("SearchProjects"),
					"request": map[string]any{
						"@type": string("type.googleapis.com/google.cloud.resourcemanager.v3.SearchProjectsRequest"),
						"query": string("state:active AND NOT projectID:sys-*"),
					},
					"requestMetadata": map[string]any{
						"callerIp":                string("21.128.18.1"),
						"callerSuppliedUserAgent": string("grpc-go/1.62.2,gzip(gfe)"),
						"destinationAttributes":   map[string]any{},
						"requestAttributes":       map[string]any{},
					},
					"resourceName": string("projects/project-id"),
					"serviceName":  string("cloudresourcemanager.googleapis.com"),
					"status":       map[string]any{},
				},
			},
		},
	}
	logger, _ := zap.NewDevelopment()
	for _, tt := range tests {
		fn := func(t *testing.T, options testOptions, want Log) {
			var errs error
			wantRes, wantLr, err := generateLog(t, want)
			errs = multierr.Append(errs, err)

			gotRes, gotLr, err := TranslateLogEntry(context.TODO(), logger, []byte(tt.input), options)
			errs = multierr.Append(errs, err)
			errs = multierr.Combine(errs, compareResources(wantRes, gotRes), compareLogRecords(wantLr, gotLr))

			require.NoError(t, errs)
		}
		var options testOptions
		t.Run(tt.scenario, func(t *testing.T) {
			fn(t, options, tt.wantAsProto)
		})
	}
}
