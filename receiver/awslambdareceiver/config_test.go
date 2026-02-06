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
			componentIDToLoad: component.NewIDWithName(metadata.Type, "awslogs_encoding"),
			expected: &Config{
				S3: sharedConfig{
					Encoding: "awslogs_encoding",
				},
				CloudWatch: sharedConfig{
					Encoding: "awslogs_encoding",
				},
			},
		},
		{
			name:              "Config with both S3 config only",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "json_log_encoding"),
			expected: &Config{
				S3: sharedConfig{
					Encoding: "json_log_encoding",
				},
				CloudWatch: sharedConfig{},
			},
		},
		{
			name:              "Config with empty encoding",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "empty_encoding"),
			expected: &Config{
				S3:         sharedConfig{},
				CloudWatch: sharedConfig{},
			},
		},
		{
			name:              "Config with failure bucket ARN",
			componentIDToLoad: component.NewIDWithName(metadata.Type, "with_failure_arn"),
			expected: &Config{
				S3:               sharedConfig{},
				CloudWatch:       sharedConfig{},
				FailureBucketARN: "arn:aws:s3:::example",
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
