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

package googlecloudpubsubreceiver

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
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[config.Type(typeStr)] = factory
	cfg, err := servicetest.LoadConfig(
		filepath.Join("testdata", "config.yaml"), factories,
	)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	defaultConfig := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, cfg.Receivers[config.NewComponentID(typeStr)], defaultConfig)

	customConfig := factory.CreateDefaultConfig().(*Config)
	customConfig.SetIDName("customname")

	customConfig.ProjectID = "my-project"
	customConfig.UserAgent = "opentelemetry-collector-contrib {{version}}"
	customConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 20 * time.Second,
	}
	customConfig.Subscription = "projects/my-project/subscriptions/otlp-subscription"
	assert.Equal(t, cfg.Receivers[config.NewComponentIDWithName(typeStr, "customname")], customConfig)
}

func TestConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	assert.Error(t, c.validateForTrace())
	assert.Error(t, c.validateForLog())
	assert.Error(t, c.validateForMetric())
	c.Subscription = "projects/000project/subscriptions/my-subscription"
	assert.Error(t, c.validate())
	c.Subscription = "projects/my-project/topics/my-topic"
	assert.Error(t, c.validate())
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())
}

func TestTraceConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validateForTrace())

	c.Encoding = "otlp_proto_metric"
	assert.Error(t, c.validateForTrace())
	c.Encoding = "otlp_proto_log"
	assert.Error(t, c.validateForTrace())
	c.Encoding = "raw_text"
	assert.Error(t, c.validateForTrace())
	c.Encoding = "raw_json"
	assert.Error(t, c.validateForTrace())

	c.Encoding = "otlp_proto_trace"
	assert.NoError(t, c.validateForTrace())
}

func TestMetricConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validateForMetric())

	c.Encoding = "otlp_proto_trace"
	assert.Error(t, c.validateForMetric())
	c.Encoding = "otlp_proto_log"
	assert.Error(t, c.validateForMetric())
	c.Encoding = "raw_text"
	assert.Error(t, c.validateForMetric())
	c.Encoding = "raw_json"
	assert.Error(t, c.validateForMetric())

	c.Encoding = "otlp_proto_metric"
	assert.NoError(t, c.validateForMetric())
}

func TestLogConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validateForLog())

	c.Encoding = "otlp_proto_trace"
	assert.Error(t, c.validateForLog())
	c.Encoding = "otlp_proto_metric"
	assert.Error(t, c.validateForLog())

	c.Encoding = "raw_text"
	assert.NoError(t, c.validateForLog())
	c.Encoding = "raw_json"
	assert.NoError(t, c.validateForLog())
	c.Encoding = "otlp_proto_log"
	assert.NoError(t, c.validateForLog())
}
