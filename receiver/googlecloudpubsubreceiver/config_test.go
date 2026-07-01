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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
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
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	// Invalid: project IDs cannot start with a numeric character.
	c.Subscription = "projects/000project/subscriptions/my-subscription"
	assert.Error(t, c.validate())

	// Invalid: Don't allow topics.
	c.Subscription = "projects/my-project/topics/my-topic"
	assert.Error(t, c.validate())

	// Invalid: colon at the start.
	c.Subscription = "projects/:invalid/subscriptions/my-subscription"
	assert.Error(t, c.validate())

	// Invalid: colon at the end.
	c.Subscription = "projects/invalid:/subscriptions/my-subscription"
	assert.Error(t, c.validate())

	// Invalid: hyphen at the end.
	c.Subscription = "projects/my-project-/subscriptions/my-subscription"
	assert.Error(t, c.validate())

	// Valid: Standard project ID subscription.
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())

	// Valid: Domain-scoped projects with colon.
	c.Subscription = "projects/s3ns:my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())

	// Valid: Domain-scoped projects with dot and colon.
	c.Subscription = "projects/example.com:my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())

	// Note: The current regex is slightly permissive and allows multiple colons.
	c.Subscription = "projects/s3ns:invalid:invalid/subscriptions/my-subscription"
	assert.NoError(t, c.validate())
}

