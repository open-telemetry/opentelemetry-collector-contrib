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

func TestS3LogsDecoderRouter_GetDecoder(t *testing.T) {
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
			router := newS3LogsDecoderRouter(tt.encodings, tt.decoders)
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

func TestS3LogsDecoderRouter_PatternPriority(t *testing.T) {
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

	router := newS3LogsDecoderRouter(encodings, decoders)

	_, name, err := router.GetDecoder("AWSLogs/123/vpcflowlogs/us-east-1/file.log")
	require.NoError(t, err)
	assert.Equal(t, "vpcflow", name, "VPC flow log should match vpcflow, not catchall")

	_, name, err = router.GetDecoder("random/file.log")
	require.NoError(t, err)
	assert.Equal(t, "catchall", name, "Random path should fall through to catchall")
}

func TestCWLogsDecoderRouter_GetDecoder(t *testing.T) {
	t.Parallel()

	defaultDecoder := internal.NewDefaultCWLogsDecoder()
	lambdaDecoder := internal.NewDefaultCWLogsDecoder()
	vpcDecoder := internal.NewDefaultCWLogsDecoder()

	tests := []struct {
		name             string
		encodings        []CWEncoding
		decoders         map[string]encoding.LogsDecoderFactory
		logGroup         string
		logStream        string
		expectedEncoding string
		expectError      bool
	}{
		{
			name: "matches log_group_pattern",
			encodings: []CWEncoding{
				{Name: "lambda", Encoding: "enc/lambda", LogGroupPattern: "/aws/lambda/*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"lambda": lambdaDecoder,
			},
			logGroup:         "/aws/lambda/my-function",
			logStream:        "2024/01/15/[$LATEST]abc123",
			expectedEncoding: "lambda",
		},
		{
			name: "matches log_stream_pattern",
			encodings: []CWEncoding{
				{Name: "vpc", Encoding: "enc/vpc", LogStreamPattern: "eni-*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpc": vpcDecoder,
			},
			logGroup:         "/my/custom/group",
			logStream:        "eni-0abc123def-all",
			expectedEncoding: "vpc",
		},
		{
			name: "log_group_pattern checked before log_stream_pattern across entries",
			encodings: []CWEncoding{
				{Name: "lambda", Encoding: "enc/lambda", LogGroupPattern: "/aws/lambda/*"},
				{Name: "vpc", Encoding: "enc/vpc", LogStreamPattern: "eni-*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"lambda": lambdaDecoder,
				"vpc":    vpcDecoder,
			},
			logGroup:         "/aws/lambda/my-function",
			logStream:        "some-stream",
			expectedEncoding: "lambda",
		},
		{
			name: "catch-all log_group_pattern matches everything",
			encodings: []CWEncoding{
				{Name: "lambda", Encoding: "enc/lambda", LogGroupPattern: "/aws/lambda/*"},
				{Name: "raw", LogGroupPattern: "*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"lambda": lambdaDecoder,
				"raw":    defaultDecoder,
			},
			logGroup:         "/custom/group",
			logStream:        "some-stream",
			expectedEncoding: "raw",
		},
		{
			name: "both patterns set: log_group_pattern matches first",
			encodings: []CWEncoding{
				{Name: "my-app", Encoding: "enc/app", LogGroupPattern: "/my-company/*", LogStreamPattern: "prod-*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"my-app": lambdaDecoder,
			},
			logGroup:         "/my-company/service-a",
			logStream:        "dev-stream",
			expectedEncoding: "my-app",
		},
		{
			name: "both patterns set: log_group miss but log_stream matches",
			encodings: []CWEncoding{
				{Name: "my-app", Encoding: "enc/app", LogGroupPattern: "/my-company/*", LogStreamPattern: "prod-*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"my-app": lambdaDecoder,
			},
			logGroup:         "/other-company/service-b",
			logStream:        "prod-stream-1",
			expectedEncoding: "my-app",
		},
		{
			name: "both patterns set: neither matches returns error",
			encodings: []CWEncoding{
				{Name: "my-app", Encoding: "enc/app", LogGroupPattern: "/my-company/*", LogStreamPattern: "prod-*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"my-app": lambdaDecoder,
			},
			logGroup:    "/other-company/service",
			logStream:   "dev-stream",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newCWLogsDecoderRouter(tt.encodings, tt.decoders)
			decoder, encodingName, err := router.GetDecoder(tt.logGroup, tt.logStream)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, decoder)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, decoder)
				assert.Equal(t, tt.expectedEncoding, encodingName)
			}
		})
	}
}

func TestCWLogsDecoderRouter_PatternPriority(t *testing.T) {
	t.Parallel()

	lambdaDecoder := internal.NewDefaultCWLogsDecoder()
	paymentDecoder := internal.NewDefaultCWLogsDecoder()
	catchallDecoder := internal.NewDefaultCWLogsDecoder()

	// Entries passed in already sorted (as sortedEncodings() would return):
	// more-specific log_group first, catch-all last.
	encodings := []CWEncoding{
		{Name: "payment-lambda", Encoding: "enc/payment", LogGroupPattern: "/aws/lambda/payment-*"},
		{Name: "lambda", Encoding: "enc/lambda", LogGroupPattern: "/aws/lambda/*"},
		{Name: "raw", LogGroupPattern: "*"},
	}
	decoders := map[string]encoding.LogsDecoderFactory{
		"payment-lambda": paymentDecoder,
		"lambda":         lambdaDecoder,
		"raw":            catchallDecoder,
	}

	router := newCWLogsDecoderRouter(encodings, decoders)

	// Payment-specific lambda matches the most-specific entry.
	_, name, err := router.GetDecoder("/aws/lambda/payment-service", "stream-1")
	require.NoError(t, err)
	assert.Equal(t, "payment-lambda", name, "payment lambda should match payment-lambda, not plain lambda")

	// Non-payment lambda falls through to the generic lambda entry.
	_, name, err = router.GetDecoder("/aws/lambda/other-service", "stream-1")
	require.NoError(t, err)
	assert.Equal(t, "lambda", name, "other lambda should match generic lambda")

	// Unrelated log group falls through to catch-all.
	_, name, err = router.GetDecoder("/custom/group", "stream-1")
	require.NoError(t, err)
	assert.Equal(t, "raw", name, "unrelated log group should fall through to catch-all")
}
