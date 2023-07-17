// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	defaultConfig := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, cfg, defaultConfig)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	customConfig := factory.CreateDefaultConfig().(*Config)

	customConfig.ProjectID = "my-project"
	customConfig.UserAgent = "opentelemetry-collector-contrib {{version}}"
	customConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 20 * time.Second,
	}
	customConfig.Topic = "projects/my-project/topics/otlp-topic"
	customConfig.Compression = "gzip"
	customConfig.Watermark.Behavior = "earliest"
	customConfig.Watermark.AllowedDrift = time.Hour
	assert.Equal(t, cfg, customConfig)
}

func TestTopicConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	assert.Error(t, c.Validate())
	c.Topic = "projects/000project/topics/my-topic"
	assert.Error(t, c.Validate())
	c.Topic = "projects/my-project/subscriptions/my-subscription"
	assert.Error(t, c.Validate())
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.Validate())
}

func TestCompressionConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.Validate())
	c.Compression = "xxx"
	assert.Error(t, c.Validate())
	c.Compression = "gzip"
	assert.NoError(t, c.Validate())
	c.Compression = "none"
	assert.Error(t, c.Validate())
	c.Compression = ""
	assert.NoError(t, c.Validate())
}

func TestWatermarkBehaviorConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.Validate())
	c.Watermark.Behavior = "xxx"
	assert.Error(t, c.Validate())
	c.Watermark.Behavior = "earliest"
	assert.NoError(t, c.Validate())
	c.Watermark.Behavior = "none"
	assert.Error(t, c.Validate())
	c.Watermark.Behavior = "current"
	assert.NoError(t, c.Validate())
}

func TestWatermarkDefaultMaxDriftValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.Validate())
	c.Watermark.AllowedDrift = 0
	assert.Equal(t, time.Duration(0), c.Watermark.AllowedDrift)
	assert.NoError(t, c.Validate())
	assert.Equal(t, time.Duration(9223372036854775807), c.Watermark.AllowedDrift)
}
