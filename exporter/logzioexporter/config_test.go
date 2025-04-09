// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := &Config{
		Token:  "token",
		Region: "eu",
	}
	expected.BackOffConfig = configretry.NewDefaultBackOffConfig()
	expected.MaxInterval = 5 * time.Second
	expected.QueueSettings = exporterhelper.NewDefaultQueueConfig()
	expected.QueueSettings.Enabled = false
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 30 * time.Second
	clientConfig.Compression = configcompression.TypeGzip
	clientConfig.WriteBufferSize = 512 * 1024
	expected.ClientConfig = clientConfig
	assert.Equal(t, expected, cfg)
}

func TestDefaultLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "configd.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := &Config{
		Token: "logzioTESTtoken",
	}
	expected.BackOffConfig = configretry.NewDefaultBackOffConfig()
	expected.QueueSettings = exporterhelper.NewDefaultQueueConfig()
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 30 * time.Second
	clientConfig.Compression = configcompression.TypeGzip
	clientConfig.WriteBufferSize = 512 * 1024
	expected.ClientConfig = clientConfig
	assert.Equal(t, expected, cfg)
}

func TestCheckAndWarnDeprecatedOptions(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 10 * time.Second
	clientConfig.Compression = configcompression.TypeGzip
	clientConfig.WriteBufferSize = 512 * 1024
	// Config with legacy options
	actualCfg := &Config{
		QueueSettings:  exporterhelper.NewDefaultQueueConfig(),
		BackOffConfig:  configretry.NewDefaultBackOffConfig(),
		Token:          "logzioTESTtoken",
		CustomEndpoint: "https://api.example.com",
		QueueMaxLength: 10,
		DrainInterval:  10,
		ClientConfig:   clientConfig,
	}
	params := exportertest.NewNopSettings(metadata.Type)
	logger := hclog2ZapLogger{
		Zap:  params.Logger,
		name: loggerName,
	}
	actualCfg.checkAndWarnDeprecatedOptions(&logger)

	clientConfigEndpoint := confighttp.NewDefaultClientConfig()
	clientConfigEndpoint.Timeout = 10 * time.Second
	clientConfigEndpoint.Compression = configcompression.TypeGzip
	clientConfigEndpoint.WriteBufferSize = 512 * 1024
	clientConfigEndpoint.Endpoint = "https://api.example.com"

	expected := &Config{
		Token:          "logzioTESTtoken",
		CustomEndpoint: "https://api.example.com",
		QueueMaxLength: 10,
		DrainInterval:  10,
		BackOffConfig:  configretry.NewDefaultBackOffConfig(),
		QueueSettings:  exporterhelper.NewDefaultQueueConfig(),
		ClientConfig:   clientConfigEndpoint,
	}
	expected.QueueSettings.QueueSize = 10
	assert.Equal(t, expected, actualCfg)
}

func TestNullTokenConfig(tester *testing.T) {
	cfg := Config{
		Region: "eu",
	}
	assert.Error(tester, cfg.Validate(), "Empty token should produce error")
}
