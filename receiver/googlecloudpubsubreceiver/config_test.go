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
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[config.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfig(
		path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	defaultConfig := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, cfg.Receivers[config.NewID(typeStr)], defaultConfig)

	customConfig := factory.CreateDefaultConfig().(*Config)
	customConfig.SetIDName("customname")

	customConfig.ProjectID = "my-project"
	customConfig.UserAgent = "opentelemetry-collector-contrib {{version}}"
	customConfig.Endpoint = "test-endpoint"
	customConfig.Insecure = true
	customConfig.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: 20 * time.Second,
	}
	customConfig.Subscription = "projects/my-project/subscriptions/otlp-subscription"
	assert.Equal(t, cfg.Receivers[config.NewIDWithName(typeStr, "customname")], customConfig)
}

func TestConfigValidation(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	assert.Error(t, config.validateForTrace())
	assert.Error(t, config.validateForLog())
	assert.Error(t, config.validateForMetric())
	config.Subscription = "projects/000project/subscriptions/my-subscription"
	assert.Error(t, config.validate())
	config.Subscription = "projects/my-project/topics/my-topic"
	assert.Error(t, config.validate())
	config.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, config.validate())
}

func TestTraceConfigValidation(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, config.validateForTrace())

	config.Encoding = "otlp_proto_metric"
	assert.Error(t, config.validateForTrace())
	config.Encoding = "otlp_proto_log"
	assert.Error(t, config.validateForTrace())
	config.Encoding = "raw_text"
	assert.Error(t, config.validateForTrace())
	config.Encoding = "raw_json"
	assert.Error(t, config.validateForTrace())

	config.Encoding = "otlp_proto_trace"
	assert.NoError(t, config.validateForTrace())
}

func TestMetricConfigValidation(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, config.validateForMetric())

	config.Encoding = "otlp_proto_trace"
	assert.Error(t, config.validateForMetric())
	config.Encoding = "otlp_proto_log"
	assert.Error(t, config.validateForMetric())
	config.Encoding = "raw_text"
	assert.Error(t, config.validateForMetric())
	config.Encoding = "raw_json"
	assert.Error(t, config.validateForMetric())

	config.Encoding = "otlp_proto_metric"
	assert.NoError(t, config.validateForMetric())
}

func TestLogConfigValidation(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, config.validateForLog())

	config.Encoding = "otlp_proto_trace"
	assert.Error(t, config.validateForLog())
	config.Encoding = "otlp_proto_metric"
	assert.Error(t, config.validateForLog())

	config.Encoding = "raw_text"
	assert.NoError(t, config.validateForLog())
	config.Encoding = "raw_json"
	assert.NoError(t, config.validateForLog())
	config.Encoding = "otlp_proto_log"
	assert.NoError(t, config.validateForLog())
}
