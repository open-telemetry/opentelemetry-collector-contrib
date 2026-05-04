// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/metadata"
)

func TestCreateLogsReceiver(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	cfg := f.CreateDefaultConfig().(*Config)

	settings := receivertest.NewNopSettings(receivertest.NopType)
	settings.ID = component.MustNewID("azure_functions")
	ext, err := f.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestCreateMetricsReceiver(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	cfg := f.CreateDefaultConfig().(*Config)

	settings := receivertest.NewNopSettings(receivertest.NopType)
	settings.ID = component.MustNewID("azure_functions")
	ext, err := f.CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, ext)
}

// TestReuseLogsAndMetricsReceiversSameConfig checks that CreateLogs and CreateMetrics with the same
// config return the same wrapped receiver instance. The collector builds separate receiver objects
// for the logs and metrics pipelines, but they must share one *functionsReceiver so Start binds the
// HTTP listener only once
func TestReuseLogsAndMetricsReceiversSameConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.HTTP.NetAddr.Endpoint = "test:123"
	encID := component.MustNewID("azure_encoding")
	cfg.Triggers = &TriggersConfig{
		EventHub: &EventHubTriggerConfig{
			Logs: []EncodingConfig{
				{Name: "logs", Encoding: encID},
			},
			Metrics: []EncodingConfig{
				{Name: "metrics", Encoding: encID},
			},
		},
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	logsConsumer := consumertest.NewNop()
	metricsConsumer := consumertest.NewNop()
	rLogs, err := createLogsReceiver(t.Context(), settings, cfg, logsConsumer)
	require.NoError(t, err)
	rMetrics, err := createMetricsReceiver(t.Context(), settings, cfg, metricsConsumer)
	require.NoError(t, err)
	assert.Equal(t, rLogs, rMetrics)
	shared := rLogs.(*sharedcomponent.SharedComponent)
	fr := shared.Unwrap().(*functionsReceiver)
	assert.NotNil(t, fr.nextLogs)
	assert.NotNil(t, fr.nextMetrics)
}
