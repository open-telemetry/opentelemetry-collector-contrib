// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

// Ensure stream create works as expected
func TestValidMetricsStream(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	uaa, err := newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedClientConfig,
		cfg.UAA.Username,
		string(cfg.UAA.Password))

	require.NoError(t, err)
	require.NotNil(t, uaa)

	streamFactory, streamErr := newEnvelopeStreamFactory(
		context.Background(),
		componenttest.NewNopTelemetrySettings(),
		uaa,
		cfg.RLPGateway.ClientConfig,
		componenttest.NewNopHost())

	require.NoError(t, streamErr)
	require.NotNil(t, streamFactory)

	innerCtx, cancel := context.WithCancel(context.Background())

	envelopeStream := streamFactory.CreateMetricsStream(innerCtx, cfg.RLPGateway.ShardID)

	require.NotNil(t, envelopeStream)

	cancel()
}

// Ensure stream create works as expected
func TestValidLogsStream(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	uaa, err := newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedClientConfig,
		cfg.UAA.Username,
		string(cfg.UAA.Password))

	require.NoError(t, err)
	require.NotNil(t, uaa)

	streamFactory, streamErr := newEnvelopeStreamFactory(
		context.Background(),
		componenttest.NewNopTelemetrySettings(),
		uaa,
		cfg.RLPGateway.ClientConfig,
		componenttest.NewNopHost())

	require.NoError(t, streamErr)
	require.NotNil(t, streamFactory)

	innerCtx, cancel := context.WithCancel(context.Background())

	envelopeStream := streamFactory.CreateLogsStream(innerCtx, cfg.RLPGateway.ShardID)

	require.NotNil(t, envelopeStream)

	cancel()
}
