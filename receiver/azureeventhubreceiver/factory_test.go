// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
