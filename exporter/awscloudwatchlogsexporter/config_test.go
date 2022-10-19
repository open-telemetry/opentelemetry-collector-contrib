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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultRetrySettings := exporterhelper.NewDefaultRetrySettings()

	tests := []struct {
		id           config.ComponentID
		expected     config.Exporter
		errorMessage string
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "e1-defaults"),
			expected: &Config{
				ExporterSettings:   config.NewExporterSettings(config.NewComponentID(typeStr)),
				RetrySettings:      defaultRetrySettings,
				LogGroupName:       "test-1",
				LogStreamName:      "testing",
				Endpoint:           "",
				AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
				QueueSettings: QueueSettings{
					QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize,
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "e2-no-retries-short-queue"),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:         false,
					InitialInterval: defaultRetrySettings.InitialInterval,
					MaxInterval:     defaultRetrySettings.MaxInterval,
					MaxElapsedTime:  defaultRetrySettings.MaxElapsedTime,
				},
				AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
				LogGroupName:       "test-2",
				LogStreamName:      "testing",
				QueueSettings: QueueSettings{
					QueueSize: 2,
				},
			},
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "invalid_queue_size"),
			errorMessage: "'sending_queue.queue_size' must be 1 or greater",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "invalid_required_field_stream"),
			errorMessage: "'log_stream_name' must be set",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "invalid_required_field_group"),
			errorMessage: "'log_group_name' must be set",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "invalid_queue_setting"),
			errorMessage: `'sending_queue' has invalid keys: enabled, num_consumers`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			err = config.UnmarshalExporter(sub, cfg)

			if tt.expected == nil {
				err = multierr.Append(err, cfg.Validate())
				assert.ErrorContains(t, err, tt.errorMessage)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
