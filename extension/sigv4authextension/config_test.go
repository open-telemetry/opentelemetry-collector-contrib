// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	awsCredsProvider := mockCredentials()
	awsCreds, _ := (*awsCredsProvider).Retrieve(context.Background())

	t.Setenv("AWS_ACCESS_KEY_ID", awsCreds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", awsCreds.SecretAccessKey)

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(component.NewID(metadata.Type).String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, &Config{
		Region:  "region",
		Service: "service",
		AssumeRole: AssumeRole{
			SessionName: "role_session_name",
			STSRegion:   "region",
		},
		// Ensure creds are the same for load config test; tested in extension_test.go
		credsProvider: cfg.(*Config).credsProvider,
	}, cfg)
}

func TestLoadWebIdentityConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "web_identity").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, &Config{
		Region:  "region",
		Service: "service",
		AssumeRole: AssumeRole{
			ARN:                  "arn:aws:iam::12345678910:role/my_role",
			WebIdentityTokenFile: "testdata/token_file",
			STSRegion:            "region",
		},
		credsProvider: cfg.(*Config).credsProvider,
	}, cfg)
}

func TestLoadConfigError(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "missing_credentials").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.Error(t, xconfmap.Validate(cfg))
}
