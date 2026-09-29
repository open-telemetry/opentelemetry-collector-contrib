// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension/internal/metadata"
)

func TestConfig_Validate(t *testing.T) {
	require.NoError(t, (&Config{Region: "us-east-1"}).Validate(), "a region is the only required field")
	require.ErrorIs(t, (&Config{}).Validate(), errNoRegion, "an empty region fails at config load")
}

func TestValidateRejectsAssumeRoleFieldsWithoutARN(t *testing.T) {
	cfg := &Config{Region: "eu-central-1", AssumeRole: AssumeRole{SessionName: "x"}}
	require.ErrorIs(t, cfg.Validate(), errAssumeRoleWithoutARN)
}

func TestValidateAcceptsAssumeRole(t *testing.T) {
	cfg := &Config{Region: "eu-central-1", AssumeRole: AssumeRole{ARN: "arn:aws:iam::123456789012:role/reader"}}
	require.NoError(t, cfg.Validate())
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: &Config{Region: "us-east-1"},
		},
		{
			id: component.NewIDWithName(metadata.Type, "assume_role"),
			expected: &Config{
				Region: "eu-central-1",
				AssumeRole: AssumeRole{
					ARN:         "arn:aws:iam::123456789012:role/db-monitor",
					SessionName: "otel-collector",
					STSRegion:   "us-east-1",
					ExternalID:  "my-external-id",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
