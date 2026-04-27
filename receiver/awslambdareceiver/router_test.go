// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

func TestLogsDecoderRouter_GetDecoder(t *testing.T) {
	t.Parallel()

	defaultDecoder := internal.NewDefaultS3LogsDecoder()
	vpcDecoder := internal.NewDefaultS3LogsDecoder()
	ctDecoder := internal.NewDefaultS3LogsDecoder()

	tests := []struct {
		name           string
		encodings      []S3Encoding
		decoders       map[string]encoding.LogsDecoderFactory
		objectKey      string
		expectedFormat string
		expectError    bool
	}{
		{
			name: "matches vpcflow default pattern",
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "cloudtrail", Encoding: "awslogs_encoding/ct"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow":    vpcDecoder,
				"cloudtrail": ctDecoder,
			},
			objectKey:      "AWSLogs/123456789012/vpcflowlogs/us-east-1/2024/01/15/file.log.gz",
			expectedFormat: "vpcflow",
		},
		{
			name: "matches cloudtrail default pattern",
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "cloudtrail", Encoding: "awslogs_encoding/ct"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow":    vpcDecoder,
				"cloudtrail": ctDecoder,
			},
			objectKey:      "AWSLogs/123456789012/CloudTrail/us-east-1/2024/01/15/file.json.gz",
			expectedFormat: "cloudtrail",
		},
		{
			name: "catch-all matches unrecognized key",
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "catchall", PathPattern: "*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow":  vpcDecoder,
				"catchall": defaultDecoder,
			},
			objectKey:      "random/path/to/file.log",
			expectedFormat: "catchall",
		},
		{
			name: "raw passthrough (no encoding) uses default decoder",
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "raw", PathPattern: "raw-logs"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow": vpcDecoder,
				"raw":     defaultDecoder,
			},
			objectKey:      "raw-logs/file.txt",
			expectedFormat: "raw",
		},
		{
			name: "no matching pattern returns error",
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow": vpcDecoder,
			},
			objectKey:   "completely/different/path/file.log",
			expectError: true,
		},
		{
			name: "custom path pattern with wildcard",
			encodings: []S3Encoding{
				{Name: "custom", Encoding: "custom_encoding", PathPattern: "my-app/*/logs"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"custom": vpcDecoder,
			},
			objectKey:      "my-app/production/logs/2024/file.log",
			expectedFormat: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newLogsDecoderRouter(tt.encodings, tt.decoders)
			decoder, formatName, err := router.GetDecoder(tt.objectKey)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, decoder)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, decoder)
				assert.Equal(t, tt.expectedFormat, formatName)
			}
		})
	}
}

func TestLogsDecoderRouter_PatternPriority(t *testing.T) {
	t.Parallel()

	vpcDecoder := internal.NewDefaultS3LogsDecoder()
	catchallDecoder := internal.NewDefaultS3LogsDecoder()

	// Formats passed in already sorted (as sortedEncodings() would return).
	encodings := []S3Encoding{
		{Name: "vpcflow"}, // default: AWSLogs/*/vpcflowlogs
		{Name: "catchall", PathPattern: "*"},
	}
	decoders := map[string]encoding.LogsDecoderFactory{
		"vpcflow":  vpcDecoder,
		"catchall": catchallDecoder,
	}

	router := newLogsDecoderRouter(encodings, decoders)

	_, name, err := router.GetDecoder("AWSLogs/123/vpcflowlogs/us-east-1/file.log")
	require.NoError(t, err)
	assert.Equal(t, "vpcflow", name, "VPC flow log should match vpcflow, not catchall")

	_, name, err = router.GetDecoder("random/file.log")
	require.NoError(t, err)
	assert.Equal(t, "catchall", name, "Random path should fall through to catchall")
}
