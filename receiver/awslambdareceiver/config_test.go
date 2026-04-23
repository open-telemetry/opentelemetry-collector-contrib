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
				CloudWatch: CloudWatchConfig{sharedConfig: sharedConfig{Encoding: "aws_logs_encoding"}},
			},
		},
		{
			name:              "Config with both S3 config only",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "json_log_encoding"),
			expected: &Config{
				S3:         S3Config{sharedConfig: sharedConfig{Encoding: "json_log_encoding"}},
				CloudWatch: CloudWatchConfig{},
			},
		},
		{
			name:              "Config with empty encoding",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "empty_encoding"),
			expected: &Config{
				S3:         S3Config{},
				CloudWatch: CloudWatchConfig{},
			},
		},
		{
			name:              "Config with failure bucket ARN",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "with_failure_arn"),
			expected: &Config{
				S3:               S3Config{},
				CloudWatch:       CloudWatchConfig{},
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
				CloudWatch: CloudWatchConfig{},
			},
		},
		{
			name:              "Config with CloudWatch multi-encoding",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "cw_multi_encoding"),
			expected: &Config{
				S3: S3Config{},
				CloudWatch: CloudWatchConfig{
					Encodings: []CWEncoding{
						{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
						{Name: "lambda", Encoding: "awslogs_encoding/lambda", LogGroupPattern: "/aws/lambda/*"},
						{Name: "raw", LogGroupPattern: "*"},
					},
				},
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

func TestCloudWatchConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  CloudWatchConfig
		wantErr string
	}{
		{
			name:   "empty config is valid",
			config: CloudWatchConfig{},
		},
		{
			name:   "single encoding is valid",
			config: CloudWatchConfig{sharedConfig: sharedConfig{Encoding: "awslogs_encoding"}},
		},
		{
			name: "known names without patterns are valid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{
					{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
					{Name: "lambda", Encoding: "awslogs_encoding/lambda"},
				},
			},
		},
		{
			name: "custom format with log_group_pattern is valid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{
					{Name: "my-app", Encoding: "custom_encoding", LogGroupPattern: "/my-company/*/logs"},
				},
			},
		},
		{
			name: "custom format with log_stream_pattern is valid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{
					{Name: "prod", Encoding: "prod_encoding", LogStreamPattern: "production-*"},
				},
			},
		},
		{
			name: "catch-all log_group_pattern is valid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{
					{Name: "raw", LogGroupPattern: "*"},
				},
			},
		},
		{
			name: "both log_group_pattern and log_stream_pattern is valid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{
					{Name: "my-app", Encoding: "custom_encoding", LogGroupPattern: "/my-company/*", LogStreamPattern: "prod-*"},
				},
			},
		},
		{
			name: "encoding and encodings are mutually exclusive",
			config: CloudWatchConfig{
				sharedConfig: sharedConfig{Encoding: "awslogs_encoding"},
				Encodings:    []CWEncoding{{Name: "vpcflow"}},
			},
			wantErr: "'encoding' and 'encodings' are mutually exclusive",
		},
		{
			name: "encoding entry without name is invalid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{{Encoding: "awslogs_encoding", LogGroupPattern: "/aws/lambda/*"}},
			},
			wantErr: "'name' is required",
		},
		{
			name: "unknown format without patterns is invalid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{{Name: "custom_format", Encoding: "my_encoding"}},
			},
			wantErr: "'log_group_pattern' or 'log_stream_pattern' is required",
		},
		{
			name: "duplicate names are invalid",
			config: CloudWatchConfig{
				Encodings: []CWEncoding{
					{Name: "lambda", Encoding: "enc/lambda1", LogGroupPattern: "/aws/lambda/*"},
					{Name: "lambda", Encoding: "enc/lambda2", LogGroupPattern: "/aws/lambda/payment-*"},
				},
			},
			wantErr: "duplicate encoding name \"lambda\"",
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

func TestCWEncodingWithDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		enc           CWEncoding
		wantLogGroup  string
		wantLogStream string
	}{
		{name: "vpcflow gets default log_stream_pattern", enc: CWEncoding{Name: "vpcflow"}, wantLogStream: "eni-*"},
		{name: "cloudtrail gets default log_stream_pattern", enc: CWEncoding{Name: "cloudtrail"}, wantLogStream: "*_CloudTrail_*"},
		{name: "lambda gets default log_group_pattern", enc: CWEncoding{Name: "lambda"}, wantLogGroup: "/aws/lambda/*"},
		{name: "waf gets default log_group_pattern", enc: CWEncoding{Name: "waf"}, wantLogGroup: "aws-waf-logs-*"},
		{name: "rds gets default log_group_pattern", enc: CWEncoding{Name: "rds"}, wantLogGroup: "/aws/rds/instance/*/*"},
		{name: "eks gets default log_group_pattern", enc: CWEncoding{Name: "eks"}, wantLogGroup: "/aws/eks/*"},
		{name: "apigateway gets default log_group_pattern", enc: CWEncoding{Name: "apigateway"}, wantLogGroup: "API-Gateway-Execution-Logs_*"},
		{name: "user log_group_pattern wins over default", enc: CWEncoding{Name: "vpcflow", LogGroupPattern: "custom"}, wantLogGroup: "custom"},
		{name: "user log_stream_pattern wins over default", enc: CWEncoding{Name: "lambda", LogStreamPattern: "custom"}, wantLogStream: "custom"},
		{name: "unknown name with no pattern yields empty", enc: CWEncoding{Name: "unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := tt.enc.withDefaults()
			require.Equal(t, tt.wantLogGroup, resolved.LogGroupPattern)
			require.Equal(t, tt.wantLogStream, resolved.LogStreamPattern)
		})
	}
}

func TestCloudWatchConfigSortedEncodings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        CloudWatchConfig
		expectedOrder []string
	}{
		{
			name:          "empty returns nil",
			config:        CloudWatchConfig{},
			expectedOrder: nil,
		},
		{
			name: "catch-all last, log_group before log_stream",
			config: CloudWatchConfig{Encodings: []CWEncoding{
				{Name: "raw", LogGroupPattern: "*"},
				{Name: "vpcflow"},
				{Name: "lambda"},
			}},
			expectedOrder: []string{"lambda", "vpcflow", "raw"},
		},
		{
			name: "log_group before log_stream",
			config: CloudWatchConfig{Encodings: []CWEncoding{
				{Name: "vpc", LogStreamPattern: "eni-*"},
				{Name: "lambda", LogGroupPattern: "/aws/lambda/*"},
			}},
			expectedOrder: []string{"lambda", "vpc"},
		},
		{
			name: "more specific log_group first",
			config: CloudWatchConfig{Encodings: []CWEncoding{
				{Name: "all-lambda", LogGroupPattern: "/aws/lambda/*"},
				{Name: "payment-lambda", LogGroupPattern: "/aws/lambda/payment-*"},
			}},
			expectedOrder: []string{"payment-lambda", "all-lambda"},
		},
		{
			name: "full three-level sort",
			config: CloudWatchConfig{Encodings: []CWEncoding{
				{Name: "raw", LogGroupPattern: "*"},
				{Name: "prod-streams", LogStreamPattern: "production-*"},
				{Name: "payment-lambda", LogGroupPattern: "/aws/lambda/payment-*"},
				{Name: "lambda", LogGroupPattern: "/aws/lambda/*"},
				{Name: "vpcflow", LogStreamPattern: "eni-*"},
			}},
			expectedOrder: []string{"payment-lambda", "lambda", "prod-streams", "vpcflow", "raw"},
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
