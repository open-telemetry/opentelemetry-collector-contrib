// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

func Test_NewFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())
}

func Test_NewLogsReceiver(t *testing.T) {
	f := NewFactory()
	receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), f.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func Test_NewLogsReceiverInvalidSemconvFeatureGates(t *testing.T) {
	const (
		emitV1     = "pkg.translator.azurelogs.EmitV1LogConventions"
		dontEmitV0 = "pkg.translator.azurelogs.DontEmitV0LogConventions"
	)
	featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
		if g.ID() != emitV1 && g.ID() != dontEmitV0 {
			return
		}
		previous := g.IsEnabled()
		t.Cleanup(func() {
			require.NoError(t, featuregate.GlobalRegistry().Set(g.ID(), previous))
		})
	})
	require.NoError(t, featuregate.GlobalRegistry().Set(emitV1, false))
	require.NoError(t, featuregate.GlobalRegistry().Set(dontEmitV0, true))

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err, "feature gates only apply when apply_semantic_conventions is enabled")
	require.NotNil(t, receiver)

	cfg = f.CreateDefaultConfig().(*Config)
	cfg.ApplySemanticConventions = true
	receiver, err = f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorContains(t, err, "pkg.translator.azurelogs.DontEmitV0LogConventions cannot be enabled without enabling pkg.translator.azurelogs.EmitV1LogConventions")
	require.Nil(t, receiver)

	// The failure must not be cached: once the gates are valid, the same config succeeds.
	require.NoError(t, featuregate.GlobalRegistry().Set(emitV1, true))
	receiver, err = f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, receiver)
}

func Test_NewMetricsReceiver(t *testing.T) {
	f := NewFactory()
	receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), f.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestFactoryWithEncodingSkipsFormatUnmarshaler(t *testing.T) {
	encodingID := component.MustNewID("myencoding")

	config := createDefaultConfig().(*Config)
	config.Connection = testConnection
	config.Encoding = &encodingID

	f := NewFactory()
	settings := receivertest.NewNopSettings(metadata.Type)

	logs, err := f.CreateLogs(t.Context(), settings, config, consumertest.NewNop())
	require.NoError(t, err)
	assert.Nil(t, logs.(*eventhubReceiver).logsUnmarshaler)

	metrics, err := f.CreateMetrics(t.Context(), settings, config, consumertest.NewNop())
	require.NoError(t, err)
	assert.Nil(t, metrics.(*eventhubReceiver).metricsUnmarshaler)

	traces, err := f.CreateTraces(t.Context(), settings, config, consumertest.NewNop())
	require.NoError(t, err)
	assert.Nil(t, traces.(*eventhubReceiver).tracesUnmarshaler)
}
