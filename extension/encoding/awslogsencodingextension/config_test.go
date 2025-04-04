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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, "cloudwatch_logs_subscription_filter"),
			expected: &Config{
				Format: formatCloudWatchLogsSubscriptionFilter,
				VPCFlowLogConfig: VPCFlowLogConfig{
					fileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "text_vpc_flow_log"),
			expected: &Config{
				Format: formatVPCFlowLog,
				VPCFlowLogConfig: VPCFlowLogConfig{
					fileFormatPlainText,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "parquet_vpc_flow_log"),
			expected: &Config{
				Format: formatVPCFlowLog,
				VPCFlowLogConfig: VPCFlowLogConfig{
					fileFormatParquet,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "invalid_vpc_flow_log"),
			expectedErr: fmt.Sprintf(
				`unsupported file format "invalid" for VPC flow log, expected one of %q`,
				supportedVPCFlowLogFileFormat,
			),
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
