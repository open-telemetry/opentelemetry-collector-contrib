// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				AWSSessionSettings: awsutil.AWSSessionSettings{
					NumberOfWorkers:       8,
					Endpoint:              "",
					RequestTimeoutSeconds: 120,
					MaxRetries:            2,
					NoVerifySSL:           false,
					ProxyAddress:          "",
					Region:                "eu-west-1",
					LocalMode:             false,
					ResourceARN:           "arn:aws:ec2:us-east1:123456789:instance/i-293hiuhe0u",
					RoleARN:               "arn:aws:iam::123456789:role/monitoring-EKS-NodeInstanceRole",
				},
				IndexedAttributes:       []string{"indexed_attr_0", "indexed_attr_1"},
				IndexAllAttributes:      false,
				LogGroupNames:           []string{"group1", "group2"},
				skipTimestampValidation: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
