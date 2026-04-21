// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		name              string
		componentIDToLoad component.ID
		expected          component.Config
	}{
		{
			name:              "Config with both S3 and CloudWatch encoding",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "aws_logs_encoding"),
			expected: &Config{
				S3:         S3Config{sharedConfig: sharedConfig{Encoding: "aws_logs_encoding"}},
				CloudWatch: sharedConfig{Encoding: "aws_logs_encoding"},
			},
		},
		{
			name:              "Config with both S3 config only",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "json_log_encoding"),
			expected: &Config{
				S3:         S3Config{sharedConfig: sharedConfig{Encoding: "json_log_encoding"}},
				CloudWatch: sharedConfig{},
			},
		},
		{
			name:              "Config with empty encoding",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "empty_encoding"),
			expected: &Config{
				S3:         S3Config{},
				CloudWatch: sharedConfig{},
			},
		},
		{
			name:              "Config with failure bucket ARN",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "with_failure_arn"),
			expected: &Config{
				S3:               S3Config{},
				CloudWatch:       sharedConfig{},
				FailureBucketARN: "arn:aws:s3:::example",
			},
		},
		{
			name:              "Config with S3 multi-encoding",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "s3_multi_encoding"),
			expected: &Config{
				S3: S3Config{
					Encodings: []S3Encoding{
						{Name: "vpcflow", Encoding: "awslogs_encoding/vpcflow"},
						{Name: "cloudtrail", Encoding: "awslogs_encoding/cloudtrail"},
						{Name: "catchall", PathPattern: "*"},
					},
				},
				CloudWatch: sharedConfig{},
			},
		},
	}

	for _, tt := range tests {
		name := strings.ReplaceAll(tt.componentIDToLoad.String(), "/", "_")
		t.Run(name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.componentIDToLoad.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			require.NoError(t, err)
			require.Equal(t, tt.expected, cfg)
		})
	}
}

func TestS3ConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  S3Config
		wantErr string
	}{
		{
			name:   "empty config is valid",
			config: S3Config{},
		},
		{
			name:   "single encoding is valid",
			config: S3Config{sharedConfig: sharedConfig{Encoding: "awslogs_encoding"}},
		},
		{
			name: "encodings with known names is valid",
			config: S3Config{
				Encodings: []S3Encoding{
					{Name: "vpcflow", Encoding: "awslogs_encoding/vpcflow"},
					{Name: "cloudtrail", Encoding: "awslogs_encoding/cloudtrail"},
				},
			},
		},
		{
			name: "encodings with custom path_pattern is valid",
			config: S3Config{
				Encodings: []S3Encoding{
					{Name: "custom", Encoding: "my_encoding", PathPattern: "custom-logs/"},
				},
			},
		},
		{
			name: "raw passthrough entry (no encoding) is valid",
			config: S3Config{
				Encodings: []S3Encoding{
					{Name: "raw", PathPattern: "raw-logs/"},
				},
			},
		},
		{
			name: "catch-all * is valid",
			config: S3Config{
				Encodings: []S3Encoding{
					{Name: "catchall", PathPattern: "*"},
				},
			},
		},
		{
			name: "encoding and encodings are mutually exclusive",
			config: S3Config{
				sharedConfig: sharedConfig{Encoding: "awslogs_encoding"},
				Encodings:    []S3Encoding{{Name: "vpcflow"}},
			},
			wantErr: "'encoding' and 'encodings' are mutually exclusive",
		},
		{
			name: "encoding entry without name is invalid",
			config: S3Config{
				Encodings: []S3Encoding{{Encoding: "awslogs_encoding", PathPattern: "logs/"}},
			},
			wantErr: "'name' is required",
		},
		{
			name: "unknown format without path_pattern is invalid",
			config: S3Config{
				Encodings: []S3Encoding{{Name: "custom_format", Encoding: "my_encoding"}},
			},
			wantErr: "'path_pattern' is required for encoding \"custom_format\"",
		},
		{
			name: "duplicate names are invalid",
			config: S3Config{
				Encodings: []S3Encoding{
					{Name: "vpcflow", Encoding: "enc/vpc1"},
					{Name: "vpcflow", Encoding: "enc/vpc2"},
				},
			},
			wantErr: "duplicate encoding name \"vpcflow\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestS3EncodingResolvePathPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		enc      S3Encoding
		expected string
	}{
		{name: "vpcflow uses default", enc: S3Encoding{Name: "vpcflow"}, expected: "AWSLogs/*/vpcflowlogs"},
		{name: "cloudtrail uses default", enc: S3Encoding{Name: "cloudtrail"}, expected: "AWSLogs/*/CloudTrail"},
		{name: "elbaccess uses default", enc: S3Encoding{Name: "elbaccess"}, expected: "AWSLogs/*/elasticloadbalancing"},
		{name: "waf uses default", enc: S3Encoding{Name: "waf"}, expected: "AWSLogs/*/WAFLogs"},
		{name: "networkfirewall uses default", enc: S3Encoding{Name: "networkfirewall"}, expected: "AWSLogs/*/network-firewall"},
		{name: "custom overrides default", enc: S3Encoding{Name: "vpcflow", PathPattern: "AWSLogs/123/vpcflowlogs"}, expected: "AWSLogs/123/vpcflowlogs"},
		{name: "unknown without path_pattern returns empty", enc: S3Encoding{Name: "unknown"}, expected: ""},
		{name: "catch-all stays as *", enc: S3Encoding{Name: "catchall", PathPattern: "*"}, expected: "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.enc.resolvePathPattern())
		})
	}
}

func TestS3ConfigSortedEncodings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        S3Config
		expectedOrder []string
	}{
		{
			name:          "empty returns nil",
			config:        S3Config{},
			expectedOrder: nil,
		},
		{
			name: "catch-all moved to last regardless of input order",
			config: S3Config{Encodings: []S3Encoding{
				{Name: "catchall", PathPattern: "*"},
				{Name: "vpcflow"},
				{Name: "cloudtrail"},
			}},
			expectedOrder: []string{"vpcflow", "cloudtrail", "catchall"},
		},
		{
			name: "specific account beats wildcard account",
			config: S3Config{Encodings: []S3Encoding{
				{Name: "vpc-any", PathPattern: "AWSLogs/*/vpcflowlogs"},
				{Name: "vpc-specific", PathPattern: "AWSLogs/123456789012/vpcflowlogs"},
			}},
			expectedOrder: []string{"vpc-specific", "vpc-any"},
		},
		{
			name: "longer pattern beats shorter",
			config: S3Config{Encodings: []S3Encoding{
				{Name: "vpc-any", PathPattern: "AWSLogs/*/vpcflowlogs"},
				{Name: "vpc-region", PathPattern: "AWSLogs/*/vpcflowlogs/us-east-1"},
			}},
			expectedOrder: []string{"vpc-region", "vpc-any"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted := tt.config.sortedEncodings()
			if tt.expectedOrder == nil {
				require.Nil(t, sorted)
				return
			}
			var names []string
			for _, e := range sorted {
				names = append(names, e.Name)
			}
			require.Equal(t, tt.expectedOrder, names)
		})
	}
}
