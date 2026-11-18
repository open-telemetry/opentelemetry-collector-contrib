// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

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
				TimeoutSettings: exporterhelper.TimeoutConfig{Timeout: 20 * time.Second},
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestTimeoutValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		timeout string
		want    time.Duration
		wantErr bool
	}{
		{name: "zero disables the deadline", timeout: "0s", want: 0},
		{name: "negative is rejected", timeout: "-1s", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			require.NoError(t, confmap.NewFromStringMap(map[string]any{"timeout": tt.timeout}).Unmarshal(cfg))
			if tt.wantErr {
				assert.Error(t, confmap.Validate(cfg))
				return
			}
			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.want, cfg.TimeoutSettings.Timeout)
		})
	}
}
