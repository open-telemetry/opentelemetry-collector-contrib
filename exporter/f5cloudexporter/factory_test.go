// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package f5cloudexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"golang.org/x/oauth2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestFactory_TestType(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, f.Type(), component.Type(metadata.Type))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ocfg, ok := factory.CreateDefaultConfig().(*Config)
	assert.True(t, ok)
	assert.Equal(t, ocfg.HTTPClientSettings.Endpoint, "")
	assert.Equal(t, ocfg.HTTPClientSettings.Timeout, 30*time.Second, "default timeout is 30 seconds")
	assert.Equal(t, ocfg.RetryConfig.Enabled, true, "default retry is enabled")
	assert.Equal(t, ocfg.RetryConfig.MaxElapsedTime, 300*time.Second, "default retry MaxElapsedTime")
	assert.Equal(t, ocfg.RetryConfig.InitialInterval, 5*time.Second, "default retry InitialInterval")
	assert.Equal(t, ocfg.RetryConfig.MaxInterval, 30*time.Second, "default retry MaxInterval")
	assert.Equal(t, ocfg.RetryConfig.Enabled, true, "default sending queue is enabled")
}

func TestFactory_CreateMetricsExporter(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://" + testutil.GetAvailableLocalAddress(t)
	cfg.Source = "tests"
	cfg.AuthConfig = AuthConfig{
		CredentialFile: "testdata/empty_credential_file.json",
		Audience:       "tests",
	}

	creationParams := exportertest.NewNopCreateSettings()
	creationParams.BuildInfo = component.BuildInfo{
		Version: "0.0.0",
	}
	oexp, err := factory.CreateMetricsExporter(context.Background(), creationParams, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)

	require.Equal(t, configopaque.String("opentelemetry-collector-contrib 0.0.0"), cfg.Headers["User-Agent"])
}

func TestFactory_CreateMetricsExporterInvalidConfig(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)

	creationParams := exportertest.NewNopCreateSettings()
	oexp, err := factory.CreateMetricsExporter(context.Background(), creationParams, cfg)
	require.Error(t, err)
	require.Nil(t, oexp)
}

func TestFactory_CreateTracesExporter(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://" + testutil.GetAvailableLocalAddress(t)
	cfg.Source = "tests"
	cfg.AuthConfig = AuthConfig{
		CredentialFile: "testdata/empty_credential_file.json",
		Audience:       "tests",
	}

	creationParams := exportertest.NewNopCreateSettings()
	creationParams.BuildInfo = component.BuildInfo{
		Version: "0.0.0",
	}
	oexp, err := factory.CreateTracesExporter(context.Background(), creationParams, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)

	require.Equal(t, configopaque.String("opentelemetry-collector-contrib 0.0.0"), cfg.Headers["User-Agent"])
}

func Test_Factory_CreateTracesExporterInvalidConfig(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)

	creationParams := exportertest.NewNopCreateSettings()
	oexp, err := factory.CreateTracesExporter(context.Background(), creationParams, cfg)
	require.Error(t, err)
	require.Nil(t, oexp)
}

func TestFactory_CreateLogsExporter(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://" + testutil.GetAvailableLocalAddress(t)
	cfg.Source = "tests"
	cfg.AuthConfig = AuthConfig{
		CredentialFile: "testdata/empty_credential_file.json",
		Audience:       "tests",
	}

	creationParams := exportertest.NewNopCreateSettings()
	creationParams.BuildInfo = component.BuildInfo{
		Version: "0.0.0",
	}
	oexp, err := factory.CreateLogsExporter(context.Background(), creationParams, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)

	require.Equal(t, configopaque.String("opentelemetry-collector-contrib 0.0.0"), cfg.Headers["User-Agent"])
}

func TestFactory_CreateLogsExporterInvalidConfig(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)

	creationParams := exportertest.NewNopCreateSettings()
	oexp, err := factory.CreateLogsExporter(context.Background(), creationParams, cfg)
	require.Error(t, err)
	require.Nil(t, oexp)
}

func TestFactory_getTokenSourceFromConfig(t *testing.T) {
	factory := newFactoryWithTokenSourceGetter(mockTokenSourceGetter)
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://" + testutil.GetAvailableLocalAddress(t)
	cfg.Source = "tests"
	cfg.AuthConfig = AuthConfig{
		CredentialFile: "testdata/empty_credential_file.json",
		Audience:       "tests",
	}

	ts, err := getTokenSourceFromConfig(cfg)
	assert.Error(t, err)
	assert.Nil(t, ts)
}

func mockTokenSourceGetter(_ *Config) (oauth2.TokenSource, error) {
	tkn := &oauth2.Token{
		AccessToken:  "",
		TokenType:    "",
		RefreshToken: "",
		Expiry:       time.Time{},
	}

	return oauth2.StaticTokenSource(tkn), nil
}
