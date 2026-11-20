// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

// TestDeprecatedIgnoreEncodingErrorWarnsOnce verifies that an explicit
// ignore_encoding_error logs the deprecation warning once per receiver, whatever
// its value, and that an absent option logs nothing.
func TestDeprecatedIgnoreEncodingErrorWarnsOnce(t *testing.T) {
	for _, value := range []bool{true, false} {
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Subscription = "projects/my-project/subscriptions/otlp"
		cfg.IgnoreEncodingError = &value

		core, logs := observer.New(zapcore.WarnLevel)
		params := receivertest.NewNopSettings(metadata.Type)
		params.Logger = zap.New(core)
		_, err := factory.CreateLogs(t.Context(), params, cfg, consumertest.NewNop())
		require.NoError(t, err)
		// a second signal on the same config reuses the receiver and must not warn again
		_, err = factory.CreateMetrics(t.Context(), params, cfg, consumertest.NewNop())
		require.NoError(t, err)

		entries := logs.FilterMessageSnippet("ignore_encoding_error is deprecated").All()
		require.Len(t, entries, 1)
		assert.Equal(t, "on_decode_error: ignore", entries[0].ContextMap()["replacement"])
		assert.Contains(t, entries[0].ContextMap()["documentation"], "#error-handling")
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Subscription = "projects/my-project/subscriptions/otlp"
	core, logs := observer.New(zapcore.WarnLevel)
	params := receivertest.NewNopSettings(metadata.Type)
	params.Logger = zap.New(core)
	_, err := factory.CreateLogs(t.Context(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.Equal(t, 0, logs.Len(), "no deprecation warning when the option is absent")
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"

	params := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateTraces(t.Context(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "traces receiver creation failed")
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"

	params := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateMetrics(t.Context(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "metrics receiver creation failed")
}

func TestCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"

	params := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateLogs(t.Context(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "logs receiver creation failed")
}

func TestEnsureReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).Subscription = "projects/my-project/subscriptions/my-subscription"
	tReceiver, err := factory.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	mReceiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, tReceiver, mReceiver)
	lReceiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, mReceiver, lReceiver)
}
