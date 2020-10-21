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
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory
	cfg, err := servicetest.LoadConfig(
		path.Join(".", "testdata", "config.yaml"), factories,
	)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	defaultConfig := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, cfg.Exporters[config.NewComponentID(typeStr)], defaultConfig)

	customConfig := factory.CreateDefaultConfig().(*Config)
	customConfig.SetIDName("customname")

	customConfig.ProjectID = "my-project"
	customConfig.UserAgent = "opentelemetry-collector-contrib {{version}}"
	customConfig.Endpoint = "test-endpoint"
	customConfig.Insecure = true
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
	assert.Error(t, c.validate())
	c.Topic = "projects/000project/topics/my-topic"
	assert.Error(t, c.validate())
	c.Topic = "projects/my-project/subscriptions/my-subscription"
	assert.Error(t, c.validate())
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.validate())
}

func TestCompressionConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.validate())
	c.Compression = "xxx"
	assert.Error(t, c.validate())
	c.Compression = "gzip"
	assert.NoError(t, c.validate())
	c.Compression = "none"
	assert.Error(t, c.validate())
	c.Compression = ""
	assert.NoError(t, c.validate())
}

func TestWatermarkBehaviorConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Topic = "projects/my-project/topics/my-topic"
	assert.NoError(t, c.validate())
	c.Watermark.Behavior = "xxx"
	assert.Error(t, c.validate())
	c.Watermark.Behavior = "earliest"
	assert.NoError(t, c.validate())
	c.Watermark.Behavior = "none"
	assert.Error(t, c.validate())
	c.Watermark.Behavior = "current"
	assert.NoError(t, c.validate())
}
