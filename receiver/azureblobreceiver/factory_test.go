// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()

	assert.NotNil(t, f)
}

func TestCreateTraces(t *testing.T) {
	f := NewFactory()
	ctx := t.Context()
	params := receivertest.NewNopSettings(metadata.Type)
	receiver, err := f.CreateTraces(ctx, params, getConfig(), consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestCreateLogs(t *testing.T) {
	f := NewFactory()
	ctx := t.Context()
	params := receivertest.NewNopSettings(metadata.Type)
	receiver, err := f.CreateLogs(ctx, params, getConfig(), consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestTracesAndLogsReceiversAreSame(t *testing.T) {
	f := NewFactory()
	ctx := t.Context()
	params := receivertest.NewNopSettings(metadata.Type)
	config := getConfig()
	logsReceiver, err := f.CreateLogs(ctx, params, config, consumertest.NewNop())
	require.NoError(t, err)

	tracesReceiver, err := f.CreateTraces(ctx, params, config, consumertest.NewNop())
	require.NoError(t, err)

	assert.Equal(t, logsReceiver, tracesReceiver)
}

func getConfig() component.Config {
	return &Config{
		Authentication:   "connection_string",
		ConnectionString: goodConnectionString,
		Logs:             LogsConfig{ContainerName: logsContainerName, Encoding: EncodingOTLPJSON},
		Traces:           TracesConfig{ContainerName: tracesContainerName, Encoding: EncodingOTLPJSON},
	}
}

func TestCreateCoordinatedEventHubHandler(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		cfg := getConfig().(*Config)
		cfg.EventHub.EndPoint = eventHubString
		cfg.EventHub.ConsumerGroup = "ingestion"
		if coordinated {
			cfg.EventHub.BlobCheckpointStore = &BlobCheckpointStoreConfig{ContainerName: "checkpoints"}
			cfg.EventHub.IgnoreMissingBlobs = true
		}
		factory := &blobReceiverFactory{}
		handler, err := factory.getEventHandler(cfg, zaptest.NewLogger(t))
		require.NoError(t, err)
		hub := handler.(*eventHubEventHandler)
		assert.Equal(t, "ingestion", hub.consumerGroup)
		assert.Equal(t, coordinated, hub.checkpointStore != nil)
		assert.Equal(t, coordinated, hub.ignoreMissingBlobs)
		if coordinated {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			require.NoError(t, handler.run(ctx))
			require.NoError(t, handler.close(t.Context()))
		}
	}
}
