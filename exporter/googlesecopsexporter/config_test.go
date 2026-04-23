// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configoptional"
)

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		config      *Config
		expectedErr string
	}{
		{
			desc: "Missing customer ID",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				DefaultLogType:        "log_type_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "customer_id is required",
		},
		{
			desc: "Valid backstory config",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "",
		},
		{
			desc: "Valid backstory config with auth extension",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				Auth: configoptional.Some(configauth.Config{
					AuthenticatorID: component.MustNewID("googleclientauth"),
				}),
			},
			expectedErr: "",
		},
		{
			desc: "Valid backstory config with raw log field",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				RawLogField:           `body["field"]`,
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "",
		},
		{
			desc: "Invalid batch request size limit",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: 0,
			},
			expectedErr: "batch_request_size_limit must be greater than 0",
		},
		{
			desc: "Invalid compression type",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				Compression:           "invalid",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "invalid compression type",
		},
		{
			desc: "Empty API",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "api is required",
		},
		{
			desc: "Invalid API",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   "invalid",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "invalid API: invalid",
		},
		{
			desc: "Chronicle API missing region",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				ProjectNumber:         "project_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "region is required for the Chronicle API",
		},
		{
			desc: "Chronicle API missing service endpoint",
			config: &Config{
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				ProjectNumber:         "project_example",
				Region:                "region_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "service_endpoint is required",
		},
		{
			desc: "Chronicle API missing project number",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Region:                "region_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "project_number is required for the Chronicle API",
		},
		{
			desc: "Valid Chronicle API config",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				ProjectNumber:         "project_example",
				Region:                "region_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
		},
		{
			desc: "Valid Chronicle API config with custom API version",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				ProjectNumber:         "project_example",
				Region:                "region_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				APIVersion:            "v1beta",
			},
		},
		{
			desc: "Invalid API version",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				ProjectNumber:         "project_example",
				Region:                "region_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				APIVersion:            "invalid",
			},
			expectedErr: "invalid api_version: invalid",
		},
		{
			desc: "Invalid collector ID",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CollectorID:           "not-a-uuid",
			},
			expectedErr: "invalid collector_id",
		},
		{
			desc: "Valid collector ID",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CollectorID:           "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			},
		},
		{
			desc: "Invalid raw log field",
			config: &Config{
				ServiceEndpoint:       "myendpoint.com",
				CustomerID:            "customer_example",
				DefaultLogType:        "log_type_example",
				RawLogField:           "invalid{{{",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "invalid raw_log_field",
		},
		{
			desc: "Invalid service endpoint URL",
			config: &Config{
				ServiceEndpoint:       "://missing-scheme",
				CustomerID:            "customer_example",
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "invalid service_endpoint",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			}
		})
	}
}
