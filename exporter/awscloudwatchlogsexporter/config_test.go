// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultRetrySettings := exporterhelper.NewDefaultRetrySettings()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "e1-defaults"),
			expected: &Config{
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
			id: component.NewIDWithName(metadata.Type, "e2-no-retries-short-queue"),
			expected: &Config{
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             false,
					InitialInterval:     defaultRetrySettings.InitialInterval,
					MaxInterval:         defaultRetrySettings.MaxInterval,
					MaxElapsedTime:      defaultRetrySettings.MaxElapsedTime,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
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
			id:           component.NewIDWithName(metadata.Type, "invalid_queue_size"),
			errorMessage: "'sending_queue.queue_size' must be 1 or greater",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_required_field_stream"),
			errorMessage: "'log_stream_name' must be set",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_required_field_group"),
			errorMessage: "'log_group_name' must be set",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_queue_setting"),
			errorMessage: `'sending_queue' has invalid keys: enabled, num_consumers`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			err = component.UnmarshalConfig(sub, cfg)

			if tt.expected == nil {
				err = multierr.Append(err, component.ValidateConfig(cfg))
				assert.ErrorContains(t, err, tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestRetentionValidateCorrect(t *testing.T) {
	defaultRetrySettings := exporterhelper.NewDefaultRetrySettings()
	cfg := &Config{
		RetrySettings:      defaultRetrySettings,
		LogGroupName:       "test-1",
		LogStreamName:      "testing",
		Endpoint:           "",
		LogRetention:       365,
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
		QueueSettings: QueueSettings{
			QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize,
		},
	}
	assert.NoError(t, component.ValidateConfig(cfg))

}

func TestRetentionValidateWrong(t *testing.T) {
	defaultRetrySettings := exporterhelper.NewDefaultRetrySettings()
	wrongcfg := &Config{
		RetrySettings:      defaultRetrySettings,
		LogGroupName:       "test-1",
		LogStreamName:      "testing",
		Endpoint:           "",
		LogRetention:       366,
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
		QueueSettings: QueueSettings{
			QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize,
		},
	}
	assert.Error(t, component.ValidateConfig(wrongcfg))

}

func TestValidateTags(t *testing.T) {
	defaultRetrySettings := exporterhelper.NewDefaultRetrySettings()

	// Create *string values for tags inputs
	basicValue := "avalue"
	wrongRegexValue := "***"
	emptyValue := ""
	tooLongValue := strings.Repeat("a", 257)

	// Create a map with no items and then one with too many items for testing
	emptyMap := make(map[string]*string)
	bigMap := make(map[string]*string)
	for i := 0; i < 51; i++ {
		bigMap[strconv.Itoa(i)] = &basicValue
	}

	tests := []struct {
		id           component.ID
		tags         map[string]*string
		errorMessage string
	}{
		{
			id:   component.NewIDWithName(metadata.Type, "validate-correct"),
			tags: map[string]*string{"basicKey": &basicValue},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "too-little-tags"),
			tags:         emptyMap,
			errorMessage: "invalid amount of items. Please input at least 1 tag or remove the tag field",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "too-many-tags"),
			tags:         bigMap,
			errorMessage: "invalid amount of items. Please input at most 50 tags",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "wrong-key-regex"),
			tags:         map[string]*string{"***": &basicValue},
			errorMessage: "key - *** does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "wrong-value-regex"),
			tags:         map[string]*string{"basicKey": &wrongRegexValue},
			errorMessage: "value - " + wrongRegexValue + " does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "key-too-short"),
			tags:         map[string]*string{"": &basicValue},
			errorMessage: "key -  has an invalid length. Please use keys with a length of 1 to 128 characters",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "key-too-long"),
			tags:         map[string]*string{strings.Repeat("a", 129): &basicValue},
			errorMessage: "key - " + strings.Repeat("a", 129) + " has an invalid length. Please use keys with a length of 1 to 128 characters",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "value-too-short"),
			tags:         map[string]*string{"basicKey": &emptyValue},
			errorMessage: "value - " + emptyValue + " has an invalid length. Please use values with a length of 1 to 256 characters",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "value-too-long"),
			tags:         map[string]*string{"basicKey": &tooLongValue},
			errorMessage: "value - " + tooLongValue + " has an invalid length. Please use values with a length of 1 to 256 characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := &Config{
				RetrySettings:      defaultRetrySettings,
				LogGroupName:       "test-1",
				LogStreamName:      "testing",
				Endpoint:           "",
				Tags:               tt.tags,
				AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
				QueueSettings: QueueSettings{
					QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize,
				},
			}
			if tt.errorMessage != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
		})
	}
}
