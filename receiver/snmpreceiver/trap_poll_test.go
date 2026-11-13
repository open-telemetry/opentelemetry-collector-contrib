// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestTrapFactorySignalSelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		poll, traps bool
	}{
		{name: "metrics only", poll: true},
		{name: "traps only", traps: true},
		{name: "combined", poll: true, traps: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]any{}
			if tc.poll {
				raw = trapPollTestOptions("v2c")
			}
			if tc.traps {
				raw["traps"] = map[string]any{"listen_address": "127.0.0.1:0"}
			}
			cfg := trapPollTestLoadConfig(t, raw)
			before := trapPollTestLoadConfig(t, raw)
			require.NoError(t, confmap.Validate(cfg))
			settings := receivertest.NewNopSettings(metadata.Type)
			factory := NewFactory()
			logs, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
			if tc.traps {
				require.NoError(t, err)
				require.NoError(t, logs.Shutdown(t.Context()))
			} else {
				require.ErrorContains(t, err, "traps must be configured")
			}
			metrics, err := factory.CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
			if tc.poll {
				require.NoError(t, err)
				require.NoError(t, metrics.Shutdown(t.Context()))
			} else {
				require.ErrorIs(t, err, errMetricRequired)
			}
			require.Equal(t, before, cfg, "signal construction must not mutate shared configuration")
		})
	}
}

func TestEffectivePollingConfigOmitsTraps(t *testing.T) {
	raw := trapPollTestOptions("v2c")
	raw["traps"] = map[string]any{}
	cfg := trapPollTestLoadConfig(t, raw)
	encoded := confmap.New()
	require.NoError(t, encoded.Marshal(cfg.effectivePollingConfig()))
	require.False(t, encoded.IsSet("traps"))
	require.NotNil(t, cfg.Traps)
}

func trapPollTestLoadConfig(t *testing.T, raw map[string]any) *Config {
	t.Helper()
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	require.NoError(t, confmap.NewFromStringMap(raw).Unmarshal(cfg))
	return cfg
}

func trapPollTestOptions(version string) map[string]any {
	options := map[string]any{
		"collection_interval": "20s", "initial_delay": "3s", "timeout": "7s",
		"endpoint": "udp://device.example:161", "version": version, "community": "device-community",
		"attributes": map[string]any{
			"direction": map[string]any{"enum": []string{"in", "out"}},
			"interface": map[string]any{"oid": "1.3.6.1.2.1.2.2.1.2"},
			"index":     map[string]any{"indexed_value_prefix": "if"},
		},
		"resource_attributes": map[string]any{
			"device":    map[string]any{"scalar_oid": "1.3.6.1.2.1.1.5.0"},
			"interface": map[string]any{"oid": "1.3.6.1.2.1.2.2.1.2"},
			"index":     map[string]any{"indexed_value_prefix": "interface"},
		},
		"metrics": map[string]any{
			"device.uptime": map[string]any{
				"description": "Device uptime", "unit": "1", "gauge": map[string]any{"value_type": "int"},
				"scalar_oids": []any{map[string]any{
					"oid": trapUptimeOID, "resource_attributes": []string{"device"},
					"attributes": []any{map[string]any{"name": "direction", "value": "in"}},
				}},
			},
			"device.octets": map[string]any{
				"description": "Interface octets", "unit": "By",
				"sum": map[string]any{"value_type": "int", "aggregation": "cumulative", "monotonic": true},
				"column_oids": []any{map[string]any{
					"oid": "1.3.6.1.2.1.2.2.1.10", "resource_attributes": []string{"interface", "index"},
					"attributes": []any{map[string]any{"name": "interface"}, map[string]any{"name": "index"}, map[string]any{"name": "direction", "value": "in"}},
				}},
			},
		},
	}
	if version == "v3" {
		options["user"] = "otel"
		options["security_level"] = "auth_priv"
		options["auth_type"] = "SHA256"
		options["auth_password"] = "auth-password"
		options["privacy_type"] = "AES"
		options["privacy_password"] = "privacy-password"
	}
	return options
}
