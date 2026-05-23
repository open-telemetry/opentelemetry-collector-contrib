// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
)

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := createDefaultConfig()

	params := receivertest.NewNopSettings(metadata.Type)

	lConsumer := consumertest.NewNop()
	lReceiver, err := factory.CreateLogs(t.Context(), params, cfg, lConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	shared, ok := lReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	lrc := shared.Component.(*receiverCreator)
	require.Same(t, lConsumer, lrc.nextLogsConsumer)
	require.Nil(t, lrc.nextMetricsConsumer)
	require.Nil(t, lrc.nextTracesConsumer)
	require.Nil(t, lrc.nextProfilesConsumer)

	mConsumer := consumertest.NewNop()
	mReceiver, err := factory.CreateMetrics(t.Context(), params, cfg, mConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	shared, ok = mReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	mrc := shared.Component.(*receiverCreator)
	require.Same(t, lrc, mrc)
	require.Same(t, lConsumer, mrc.nextLogsConsumer)
	require.Same(t, mConsumer, mrc.nextMetricsConsumer)
	require.Nil(t, lrc.nextTracesConsumer)
	require.Nil(t, lrc.nextProfilesConsumer)

	tConsumer := consumertest.NewNop()
	tReceiver, err := factory.CreateTraces(t.Context(), params, cfg, tConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	shared, ok = tReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	trc := shared.Component.(*receiverCreator)
	require.Same(t, mrc, trc)
	require.Same(t, lConsumer, mrc.nextLogsConsumer)
	require.Same(t, mConsumer, mrc.nextMetricsConsumer)
	require.Same(t, tConsumer, mrc.nextTracesConsumer)
	require.Nil(t, trc.nextProfilesConsumer)

	pConsumer := consumertest.NewNop()
	pReceiver, err := factory.(xreceiver.Factory).CreateProfiles(t.Context(), params, cfg, pConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, pReceiver, "receiver creation failed")

	shared, ok = pReceiver.(*sharedcomponent.SharedComponent)
	require.True(t, ok)
	prc := shared.Component.(*receiverCreator)
	require.Same(t, trc, prc)
	require.Same(t, lConsumer, prc.nextLogsConsumer)
	require.Same(t, mConsumer, prc.nextMetricsConsumer)
	require.Same(t, tConsumer, prc.nextTracesConsumer)
	require.Same(t, pConsumer, prc.nextProfilesConsumer)
}
