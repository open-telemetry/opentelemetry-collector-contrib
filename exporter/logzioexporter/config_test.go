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
	"go.opentelemetry.io/collector/config/configopaque"
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := &Config{
		Token:  "token",
		Region: "eu",
	}
	expected.BackOffConfig = configretry.NewDefaultBackOffConfig()
	expected.BackOffConfig.MaxInterval = 5 * time.Second
	expected.QueueSettings = exporterhelper.NewDefaultQueueSettings()
	expected.QueueSettings.Enabled = false
	expected.HTTPClientSettings = confighttp.HTTPClientSettings{
		Endpoint: "",
		Timeout:  30 * time.Second,
		Headers:  map[string]configopaque.String{},
		// Default to gzip compression
		Compression: configcompression.Gzip,
		// We almost read 0 bytes, so no need to tune ReadBufferSize.
		WriteBufferSize: 512 * 1024,
	}
	assert.Equal(t, expected, cfg)
}

func TestDefaultLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "configd.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := &Config{
		Token: "logzioTESTtoken",
	}
	expected.BackOffConfig = configretry.NewDefaultBackOffConfig()
	expected.QueueSettings = exporterhelper.NewDefaultQueueSettings()
	expected.HTTPClientSettings = confighttp.HTTPClientSettings{
		Endpoint: "",
		Timeout:  30 * time.Second,
		Headers:  map[string]configopaque.String{},
		// Default to gzip compression
		Compression: configcompression.Gzip,
		// We almost read 0 bytes, so no need to tune ReadBufferSize.
		WriteBufferSize: 512 * 1024,
	}
	assert.Equal(t, expected, cfg)
}

func TestCheckAndWarnDeprecatedOptions(t *testing.T) {
	// Config with legacy options
	actualCfg := &Config{
		QueueSettings:  exporterhelper.NewDefaultQueueSettings(),
		BackOffConfig:  configretry.NewDefaultBackOffConfig(),
		Token:          "logzioTESTtoken",
		CustomEndpoint: "https://api.example.com",
		QueueMaxLength: 10,
		DrainInterval:  10,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
			Timeout:  10 * time.Second,
			Headers:  map[string]configopaque.String{},
			// Default to gzip compression
			Compression: configcompression.Gzip,
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
	}
	params := exportertest.NewNopCreateSettings()
	logger := hclog2ZapLogger{
		Zap:  params.Logger,
		name: loggerName,
	}
	actualCfg.checkAndWarnDeprecatedOptions(&logger)

	expected := &Config{
		Token:          "logzioTESTtoken",
		CustomEndpoint: "https://api.example.com",
		QueueMaxLength: 10,
		DrainInterval:  10,
		BackOffConfig:  configretry.NewDefaultBackOffConfig(),
		QueueSettings:  exporterhelper.NewDefaultQueueSettings(),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://api.example.com",
			Timeout:  10 * time.Second,
			Headers:  map[string]configopaque.String{},
			// Default to gzip compression
			Compression: configcompression.Gzip,
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
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
