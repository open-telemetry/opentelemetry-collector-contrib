// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchlogsexporter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	defaultRetrySettings := exporterhelper.DefaultRetrySettings()

	e1 := cfg.Exporters[config.NewIDWithName(typeStr, "e1-defaults")].(*Config)

	assert.Equal(t,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewIDWithName(typeStr, "e1-defaults")),
			RetrySettings:    defaultRetrySettings,
			LogGroupName:     "test-1",
			LogStreamName:    "testing",
			Region:           "",
			Endpoint:         "",
			QueueSettings: QueueSettings{
				QueueSize: exporterhelper.DefaultQueueSettings().QueueSize,
			},
		},
		e1,
	)

	e2 := cfg.Exporters[config.NewIDWithName(typeStr, "e2-no-retries-short-queue")].(*Config)

	assert.Equal(t,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewIDWithName(typeStr, "e2-no-retries-short-queue")),
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:         false,
				InitialInterval: defaultRetrySettings.InitialInterval,
				MaxInterval:     defaultRetrySettings.MaxInterval,
				MaxElapsedTime:  defaultRetrySettings.MaxElapsedTime,
			},
			LogGroupName:  "test-2",
			LogStreamName: "testing",
			QueueSettings: QueueSettings{
				QueueSize: 2,
			},
		},
		e2,
	)
}

func TestFailedLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	_, err = configtest.LoadConfigAndValidate(path.Join(".", "testdata", "missing_required_field_1_config.yaml"), factories)
	assert.EqualError(t, err, "exporter \"awscloudwatchlogs\" has invalid configuration: 'log_stream_name' must be set")

	_, err = configtest.LoadConfigAndValidate(path.Join(".", "testdata", "missing_required_field_2_config.yaml"), factories)
	assert.EqualError(t, err, "exporter \"awscloudwatchlogs\" has invalid configuration: 'log_group_name' must be set")

	_, err = configtest.LoadConfigAndValidate(path.Join(".", "testdata", "invalid_queue_size.yaml"), factories)
	assert.EqualError(t, err, "exporter \"awscloudwatchlogs\" has invalid configuration: 'sending_queue.queue_size' must be 1 or greater")

	_, err = configtest.LoadConfigAndValidate(path.Join(".", "testdata", "invalid_queue_setting.yaml"), factories)
	assert.EqualError(t, err, "error reading exporters configuration for awscloudwatchlogs: 1 error(s) decoding:\n\n* 'sending_queue' has invalid keys: enabled, num_consumers")
}
