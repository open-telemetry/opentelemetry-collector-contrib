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

// TestReuseLogsAndMetricsReceiversSameConfig asserts logs and metrics pipelines that use the same
// receiver config share one underlying *functionsReceiver (one HTTP listener on Start).
func TestReuseLogsAndMetricsReceiversSameConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.HTTP.NetAddr.Endpoint = "test:123"
	enc := component.MustNewID("azure_encoding")
	cfg.Triggers = &TriggersConfig{
		EventHub: &EventHubTriggerConfig{
			Logs:    []EncodingConfig{{Name: "logs", Encoding: enc}},
			Metrics: []EncodingConfig{{Name: "metrics", Encoding: enc}},
		},
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	rLogs, err := f.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	rMetrics, err := f.CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)

	assert.Same(t, rLogs, rMetrics)
	shared := rLogs.(*sharedcomponent.SharedComponent)
	fr := shared.Unwrap().(*functionsReceiver)
	assert.NotNil(t, fr.nextLogs)
	assert.NotNil(t, fr.nextMetrics)
}
