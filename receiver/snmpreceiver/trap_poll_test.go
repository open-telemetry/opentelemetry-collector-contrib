// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestPollFactorySignalSelectionAndWarnings(t *testing.T) {
	for _, tc := range []struct {
		name        string
		legacy      bool
		poll, traps bool
	}{
		{name: "legacy metrics", legacy: true, poll: true},
		{name: "nested metrics", poll: true},
		{name: "traps only", traps: true},
		{name: "legacy combined", legacy: true, poll: true, traps: true},
		{name: "nested combined", poll: true, traps: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]any{}
			if tc.poll {
				if tc.legacy {
					raw = pollTestOptions("v2c")
				} else {
					raw["poll"] = pollTestOptions("v2c")
				}
			}
			if tc.traps {
				raw["traps"] = map[string]any{"listen_address": "127.0.0.1:0"}
			}
			cfg := pollTestLoadConfig(t, raw)
			before := pollTestLoadConfig(t, raw)
			require.NoError(t, confmap.Validate(cfg))
			core, observed := observer.New(zap.WarnLevel)
			settings := receivertest.NewNopSettings(metadata.Type)
			settings.Logger = zap.New(core)
			factory := NewFactory()
			logs, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
			if tc.traps {
				require.NoError(t, err)
				require.NoError(t, logs.Shutdown(t.Context()))
			} else {
				require.ErrorContains(t, err, "traps must be configured")
			}
			require.Zero(t, observed.Len(), "creating the logs signal must not warn about polling syntax")
			metrics, err := factory.CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
			if tc.poll {
				require.NoError(t, err)
				require.NoError(t, metrics.Shutdown(t.Context()))
			} else {
				require.ErrorIs(t, err, errMetricRequired)
			}
			if tc.legacy && tc.poll {
				require.Len(t, observed.All(), 1)
				require.Equal(t, zap.WarnLevel, observed.All()[0].Level)
				require.Equal(t, legacyPollingConfigWarning, observed.All()[0].Message)
			} else {
				require.Zero(t, observed.Len())
			}
			require.Equal(t, before, cfg, "signal construction must not mutate shared configuration")
		})
	}
}

func TestEmptyPollWithTrapsRequiresMetrics(t *testing.T) {
	cfg := pollTestLoadConfig(t, map[string]any{"poll": map[string]any{}, "traps": map[string]any{}})
	require.ErrorIs(t, confmap.Validate(cfg), errMetricRequired)
}

func TestEffectivePollConfigOmitsTraps(t *testing.T) {
	cfg := pollTestLoadConfig(t, map[string]any{"poll": pollTestOptions("v2c"), "traps": map[string]any{}})
	require.Nil(t, cfg.effectivePollConfig().Traps)
	require.NotNil(t, cfg.Traps)
}
