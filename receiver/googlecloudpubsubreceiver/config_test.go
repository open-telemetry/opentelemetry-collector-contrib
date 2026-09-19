// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				FlowControlConfig: FlowControlConfig{
					TriggerAckBatchDuration: 10 * time.Second,
					StreamAckDeadline:       60 * time.Second,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 20 * time.Second,
				},
				Subscription: "projects/my-project/subscriptions/otlp-subscription",
				FlowControlConfig: FlowControlConfig{
					TriggerAckBatchDuration: 10 * time.Second,
					StreamAckDeadline:       60 * time.Second,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "sovereign"),
			expected: &Config{
				ProjectID:      "my-sovereign-project",
				Subscription:   "projects/my-sovereign-project/subscriptions/otlp-subscription",
				UniverseDomain: "apis.example.com",
				FlowControlConfig: FlowControlConfig{
					TriggerAckBatchDuration: 10 * time.Second,
					StreamAckDeadline:       60 * time.Second,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "errorhandling"),
			expected: &Config{
				ProjectID:       "my-project",
				Subscription:    "projects/my-project/subscriptions/otlp-subscription",
				OnDecodeError:   "nack",
				OnPipelineError: "nack",
				FlowControlConfig: FlowControlConfig{
					TriggerAckBatchDuration: 10 * time.Second,
					TriggerAckBatchSize:     1000,
					StreamAckDeadline:       60 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/000project/subscriptions/my-subscription"
	assert.Error(t, c.validate())
	c.Subscription = "projects/my-project/topics/my-topic"
	assert.Error(t, c.validate())
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())
	// Test for project IDs with a single colon (not at start, not at end)
	c.Subscription = "projects/s3ns:my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())
	// Invalid: colon at the start
	c.Subscription = "projects/:invalid/subscriptions/my-subscription"
	assert.Error(t, c.validate())
	// Invalid: colon at the end
	c.Subscription = "projects/invalid:/subscriptions/my-subscription"
	assert.Error(t, c.validate())
	// Invalid: multiple colons
	c.Subscription = "projects/s3ns:invalid:invalid/subscriptions/my-subscription"
	assert.Error(t, c.validate())
}

func TestConfigValidationErrorPolicies(t *testing.T) {
	newConfig := func() *Config {
		c := NewFactory().CreateDefaultConfig().(*Config)
		c.Subscription = "projects/my-project/subscriptions/my-subscription"
		return c
	}

	c := newConfig()
	assert.NoError(t, c.validate())

	// on_decode_error accepts propagate, ignore and nack
	for _, policy := range []string{"propagate", "ignore", "nack"} {
		c = newConfig()
		c.OnDecodeError = policy
		assert.NoError(t, c.validate())
	}
	c = newConfig()
	c.OnDecodeError = "drop"
	assert.ErrorContains(t, c.validate(), "on_decode_error")

	// the deprecated ignore_encoding_error maps to ignore, and conflicts with
	// any other explicitly set on_decode_error
	c = newConfig()
	c.IgnoreEncodingError = true
	assert.NoError(t, c.validate())
	assert.Equal(t, "ignore", c.decodeErrorPolicy())
	c.OnDecodeError = "ignore"
	assert.NoError(t, c.validate())
	c.OnDecodeError = "propagate"
	assert.ErrorContains(t, c.validate(), "ignore_encoding_error conflicts with on_decode_error")
	c.OnDecodeError = "nack"
	assert.ErrorContains(t, c.validate(), "ignore_encoding_error conflicts with on_decode_error")

	// on_pipeline_error accepts ack and nack
	for _, policy := range []string{"ack", "nack"} {
		c = newConfig()
		c.OnPipelineError = policy
		assert.NoError(t, c.validate())
	}
	c = newConfig()
	c.OnPipelineError = "propagate"
	assert.ErrorContains(t, c.validate(), "on_pipeline_error")
}

func TestDecodeErrorPolicyDefaults(t *testing.T) {
	c := NewFactory().CreateDefaultConfig().(*Config)
	assert.Equal(t, "ignore", c.decodeErrorPolicy())
	c.OnDecodeError = "propagate"
	assert.Equal(t, "propagate", c.decodeErrorPolicy())
	c.OnDecodeError = ""
	c.IgnoreEncodingError = true
	assert.Equal(t, "ignore", c.decodeErrorPolicy())
	c.OnDecodeError = "nack"
	assert.Equal(t, "nack", c.decodeErrorPolicy())
}

func TestConfigValidationTriggerAckBatchSize(t *testing.T) {
	c := NewFactory().CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	// 0 (the default) disables the size trigger
	assert.Equal(t, 0, c.FlowControlConfig.TriggerAckBatchSize)
	assert.NoError(t, c.validate())
	c.FlowControlConfig.TriggerAckBatchSize = 1000
	assert.NoError(t, c.validate())
	c.FlowControlConfig.TriggerAckBatchSize = -1
	assert.ErrorContains(t, c.validate(), "trigger_ack_batch_size")
}
