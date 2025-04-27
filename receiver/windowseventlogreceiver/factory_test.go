// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
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

func TestCreateAndShutdown(t *testing.T) {
	factory := NewFactory()
	defaultConfig := factory.CreateDefaultConfig()
	cfg := defaultConfig.(*WindowsLogConfig) // This cast should work on all platforms.
	cfg.InputConfig.Channel = "Application"  // Must be explicitly set to a valid channel.

	ctx := context.Background()
	settings := receivertest.NewNopSettings(metadata.Type)
	sink := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogs(ctx, settings, cfg, sink)

	if runtime.GOOS != "windows" {
		assert.Error(t, err)
		assert.IsType(t, pipeline.ErrSignalNotSupported, err)
		assert.Nil(t, receiver)
	} else {
		assert.NoError(t, err)
		require.NotNil(t, receiver)
		require.NoError(t, receiver.Shutdown(ctx))
	}
}
