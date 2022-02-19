// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudpubsubexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory
	cfg, err := servicetest.LoadConfig(
		filepath.Join("testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	defaultConfig := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, cfg.Exporters[config.NewComponentID(typeStr)], defaultConfig)

	customConfig := factory.CreateDefaultConfig().(*Config)
	customConfig.SetIDName("customname")

	customConfig.ProjectID = "my-project"
	customConfig.UserAgent = "opentelemetry-collector-contrib {{version}}"
	customConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 20 * time.Second,
	}
	customConfig.Topic = "projects/my-project/topics/otlp-topic"
	customConfig.Compression = "gzip"
	customConfig.Watermark.Behavior = "earliest"
	customConfig.Watermark.AllowedDrift = time.Hour
	assert.Equal(t, cfg.Exporters[config.NewComponentIDWithName(typeStr, "customname")], customConfig)
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
