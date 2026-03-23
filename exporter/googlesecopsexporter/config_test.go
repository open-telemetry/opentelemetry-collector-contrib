// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		config      *Config
		expectedErr string
	}{
		{
			desc: "Both creds_file_path and creds are set",
			config: &Config{
				CredsFilePath:         "/path/to/creds_file",
				Creds:                 "creds_example",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				CustomerID:            "customer_id_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "can only specify creds_file_path or creds",
		},
		{
			desc: "Valid backstory config with creds",
			config: &Config{
				Creds:                 "creds_example",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   backstoryAPI,
				CustomerID:            "customer_id_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "",
		},
		{
			desc: "Valid backstory config with creds_file_path",
			config: &Config{
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   backstoryAPI,
				CustomerID:            "customer_id_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "",
		},
		{
			desc: "Valid backstory config with raw log field",
			config: &Config{
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				RawLogField:           `body["field"]`,
				Compression:           noCompression,
				API:                   backstoryAPI,
				CustomerID:            "customer_id_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "",
		},
		{
			desc: "Invalid batch request size limit",
			config: &Config{
				Creds:                 "creds_example",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   backstoryAPI,
				CustomerID:            "customer_id_example",
				BatchRequestSizeLimit: 0,
			},
			expectedErr: "positive batch request size limit is required",
		},
		{
			desc: "Invalid compression type",
			config: &Config{
				CredsFilePath:  "/path/to/creds_file",
				DefaultLogType: "log_type_example",
				Compression:    "invalid",
				CustomerID:     "customer_id_example",
			},
			expectedErr: "invalid compression type",
		},
		{
			desc: "Hostname contains protocol prefix",
			config: &Config{
				Hostname:              "https://myendpoint.com",
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
			expectedErr: "host should not contain a protocol prefix",
		},
		{
			desc: "Empty API",
			config: &Config{
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
			expectedErr: "api is required",
		},
		{
			desc: "Invalid API",
			config: &Config{
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   "invalid",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
			expectedErr: "invalid API: invalid",
		},
		{
			desc: "Chronicle API missing location",
			config: &Config{
				Hostname:              "myendpoint.com",
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Compression:           noCompression,
				ProjectNumber:         "project_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
			expectedErr: "location is required for the Chronicle API",
		},
		{
			desc: "Chronicle API missing hostname",
			config: &Config{
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Compression:           noCompression,
				ProjectNumber:         "project_example",
				Location:              "location_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
			expectedErr: "hostname is required for the Chronicle API",
		},
		{
			desc: "Chronicle API missing project number",
			config: &Config{
				Hostname:              "myendpoint.com",
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Compression:           noCompression,
				Location:              "location_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
			expectedErr: "project number is required for the Chronicle API",
		},
		{
			desc: "Valid Chronicle API config",
			config: &Config{
				Hostname:              "myendpoint.com",
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Compression:           noCompression,
				ProjectNumber:         "project_example",
				Location:              "location_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CustomerID:            "customer_id_example",
			},
		},
		{
			desc: "Valid Chronicle API config with custom API version",
			config: &Config{
				Hostname:              "myendpoint.com",
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Compression:           noCompression,
				ProjectNumber:         "project_example",
				Location:              "location_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				APIVersion:            "v1beta",
				CustomerID:            "customer_id_example",
			},
		},
		{
			desc: "Invalid API version",
			config: &Config{
				Hostname:              "myendpoint.com",
				CredsFilePath:         "/path/to/creds_file",
				DefaultLogType:        "log_type_example",
				API:                   chronicleAPI,
				Compression:           noCompression,
				ProjectNumber:         "project_example",
				Location:              "location_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				APIVersion:            "invalid",
				CustomerID:            "customer_id_example",
			},
			expectedErr: "invalid API version: invalid",
		},
		{
			desc: "Invalid collector ID",
			config: &Config{
				Creds:                 "creds_example",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
				CollectorID:           "not-a-uuid",
				CustomerID:            "customer_id_example",
			},
			expectedErr: "invalid collector ID",
		},
		{
			desc: "Invalid raw log field",
			config: &Config{
				Creds:                 "creds_example",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   backstoryAPI,
				RawLogField:           "invalid",
				CustomerID:            "customer_id_example",
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "invalid raw_log_field: error while parsing arguments",
		},
		{
			desc: "Missing customer ID",
			config: &Config{
				Creds:                 "creds_example",
				DefaultLogType:        "log_type_example",
				Compression:           noCompression,
				API:                   backstoryAPI,
				BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
			},
			expectedErr: "customer ID is required",
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
