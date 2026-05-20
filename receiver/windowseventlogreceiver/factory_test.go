// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	t.Run("NewFactoryCorrectType", func(t *testing.T) {
		factory := NewFactory()
		require.Equal(t, metadata.Type, factory.Type())
	})
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
}

func TestNegativeCacheSizeRejected(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cm := confmap.NewFromStringMap(map[string]any{
		"resolve_sids": map[string]any{
			"cache_size": -1,
		},
	})
	err := cm.Unmarshal(cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "cache_size")
}

func TestNegativeCacheTTLRejected(t *testing.T) {
	cfg := &ResolveSIDsConfig{
		Enabled:  true,
		CacheTTL: -1 * time.Second,
	}
	err := cfg.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "cache_ttl must not be negative")
}

func TestZeroCacheTTLAccepted(t *testing.T) {
	cfg := &ResolveSIDsConfig{
		Enabled:  true,
		CacheTTL: 0,
	}
	require.NoError(t, cfg.Validate())
}

func TestCreateAndShutdown(t *testing.T) {
	factory := NewFactory()
	defaultConfig := factory.CreateDefaultConfig()
	cfg := defaultConfig.(*WindowsLogConfig) // This cast should work on all platforms.
	cfg.InputConfig.Channel = "Application"  // Must be explicitly set to a valid channel.

	ctx := t.Context()
	settings := receivertest.NewNopSettings(metadata.Type)
	sink := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogs(ctx, settings, cfg, sink)

	if runtime.GOOS != "windows" {
		assert.Error(t, err)
		assert.ErrorContains(t, err, "windows eventlog receiver is only supported on Windows")
		assert.Nil(t, receiver)
	} else {
		assert.NoError(t, err)
		require.NotNil(t, receiver)
		require.NoError(t, receiver.Shutdown(ctx))
	}
}
