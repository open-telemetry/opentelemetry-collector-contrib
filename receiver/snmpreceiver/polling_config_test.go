// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestPollFactoryDefaultingDoesNotMutateSource(t *testing.T) {
	raw := map[string]any{
		"endpoint": "localhost",
		"metrics": map[string]any{
			"device.gauge": map[string]any{"gauge": map[string]any{}, "scalar_oids": []any{map[string]any{"oid": "1.3.6.1.2.1.1.3.0"}}},
			"device.sum":   map[string]any{"sum": map[string]any{"monotonic": true}, "scalar_oids": []any{map[string]any{"oid": "1.3.6.1.2.1.1.3.0"}}},
		},
	}
	cfg := trapPollTestLoadConfig(t, raw)
	before := trapPollTestLoadConfig(t, raw)
	metrics, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, metrics.Shutdown(t.Context()))
	require.Equal(t, before, cfg, "endpoint and nested gauge/sum defaults must only change the effective copy")
	selected := cfg.effectivePollingConfig()
	require.Equal(t, "localhost", selected.Endpoint)
	require.Empty(t, selected.Metrics["device.gauge"].Unit)
	require.Empty(t, selected.Metrics["device.gauge"].Gauge.ValueType)
	require.Empty(t, selected.Metrics["device.sum"].Sum.ValueType)
	require.Empty(t, selected.Metrics["device.sum"].Sum.Aggregation)
}
