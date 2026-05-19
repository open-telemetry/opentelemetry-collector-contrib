// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
	vpcflowlog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id:          component.NewIDWithName(metadata.Type, ""),
			expectedErr: fmt.Sprintf("format unspecified, expected one of %q", supportedLogFormats),
		},
		{
			id: component.NewIDWithName(metadata.Type, "cloudwatch"),
			expected: &Config{
				Format: constants.FormatCloudWatchLogsSubscriptionFilter,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "text_vpcflow"),
			expected: &Config{
				Format: constants.FormatVPCFlowLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "parquet_vpcflow"),
			expected: &Config{
				Format: constants.FormatVPCFlowLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatParquet,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "invalid_vpcflow"),
			expectedErr: fmt.Sprintf(
				`unsupported file format "invalid" for VPC flow log, expected one of %q`,
				supportedVPCFlowLogFileFormat,
			),
		},
		{
			id: component.NewIDWithName(metadata.Type, "s3access"),
			expected: &Config{
				Format: constants.FormatS3AccessLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "waf"),
			expected: &Config{
				Format: constants.FormatWAFLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "cloudtrail"),
			expected: &Config{
				Format: constants.FormatCloudTrailLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "elbaccess"),
			expected: &Config{
				Format: constants.FormatELBAccessLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "networkfirewall"),
			expected: &Config{
				Format: constants.FormatNetworkFirewallLog,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "cw_routing"),
			expected: &Config{
				Format: constants.FormatCloudWatchLogsSubscriptionFilter,
				VPCFlowLogConfig: vpcflowlog.Config{
					FileFormat: constants.FileFormatPlainText,
				},
				CloudWatch: CloudWatchConfig{
					Streams: []subscriptionfilter.CloudWatchStream{
						{
							Name:     "vpcflow",
							Encoding: component.MustNewIDWithName("aws_logs_encoding", "vpcflow_inner"),
						},
						{
							Name:            "payment-lambda",
							LogGroupPattern: "/aws/lambda/payment-*",
							Encoding:        component.MustNewIDWithName("aws_logs_encoding", "lambda_inner"),
						},
						{
							Name:            "catchall",
							LogGroupPattern: "*",
							Encoding:        component.MustNewIDWithName("aws_logs_encoding", "raw"),
						},
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "cw_routing_invalid_format"),
			expectedErr: `'cloudwatch.streams' is only valid with format "cloudwatch"; got "vpcflow"`,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "cw_routing_missing_name"),
			expectedErr: `cloudwatch.streams[0]: 'name' is required`,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "cw_routing_missing_encoding"),
			expectedErr: `cloudwatch.streams[0]: 'encoding' is required`,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "cw_routing_unknown_name"),
			expectedErr: `cloudwatch.streams[0]: name "not-a-known-service" has no defaults; set 'log_group_pattern' or 'log_stream_pattern'`,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "cw_routing_duplicate_name"),
			expectedErr: `cloudwatch.streams[1]: duplicate name "vpcflow" also at cloudwatch.streams[0]`,
		},
	}

	for _, tt := range tests {
		name := strings.ReplaceAll(tt.id.String(), "/", "_")
		t.Run(name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
