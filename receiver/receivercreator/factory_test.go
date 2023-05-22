// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := createDefaultConfig()

	params := receivertest.NewNopCreateSettings()

	lConsumer := consumertest.NewNop()
	lReceiver, err := factory.CreateLogsReceiver(context.Background(), params, cfg, lConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	shared, ok := lReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	lrc := shared.Component.(*receiverCreator)
	require.Same(t, lConsumer, lrc.nextLogsConsumer)
	require.Nil(t, lrc.nextMetricsConsumer)
	require.Nil(t, lrc.nextTracesConsumer)

	mConsumer := consumertest.NewNop()
	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, mConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	shared, ok = mReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	mrc := shared.Component.(*receiverCreator)
	require.Same(t, lrc, mrc)
	require.Same(t, lConsumer, mrc.nextLogsConsumer)
	require.Same(t, mConsumer, mrc.nextMetricsConsumer)
	require.Nil(t, lrc.nextTracesConsumer)

	tConsumer := consumertest.NewNop()
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), params, cfg, tConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	shared, ok = tReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	trc := shared.Component.(*receiverCreator)
	require.Same(t, mrc, trc)
	require.Same(t, lConsumer, mrc.nextLogsConsumer)
	require.Same(t, mConsumer, mrc.nextMetricsConsumer)
	require.Same(t, tConsumer, mrc.nextTracesConsumer)
}
