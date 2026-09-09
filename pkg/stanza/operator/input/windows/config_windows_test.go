// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/metadata"
)

func TestBuildEventDrivenScraping(t *testing.T) {
	gateID := metadata.StanzaWindowsEventDrivenScrapingFeatureGate.ID()

	tests := []struct {
		name        string
		gateEnabled bool
		configFlag  bool
		want        bool
	}{
		{name: "neither", gateEnabled: false, configFlag: false, want: false},
		{name: "config only", gateEnabled: false, configFlag: true, want: true},
		{name: "gate only", gateEnabled: true, configFlag: false, want: true},
		{name: "both", gateEnabled: true, configFlag: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, featuregate.GlobalRegistry().Set(gateID, tt.gateEnabled))
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(gateID, false))
			})

			cfg := NewConfig()
			cfg.Channel = "Application"
			cfg.EventDrivenScraping = tt.configFlag

			op, err := cfg.Build(component.TelemetrySettings{Logger: zap.NewNop()})
			require.NoError(t, err)
			input, ok := op.(*Input)
			require.True(t, ok)
			require.Equal(t, tt.want, input.eventDrivenScraping)
		})
	}
}

// TestBuildMaxEventsPerPollForcedZeroWhenEventDriven ensures that max_events_per_poll
// is forced to zero whenever event-driven scraping is enabled (via either the config
// option or the feature gate).
func TestBuildMaxEventsPerPollForcedZeroWhenEventDriven(t *testing.T) {
	gateID := metadata.StanzaWindowsEventDrivenScrapingFeatureGate.ID()

	t.Run("config flag forces zero", func(t *testing.T) {
		require.NoError(t, featuregate.GlobalRegistry().Set(gateID, false))

		cfg := NewConfig()
		cfg.Channel = "Application"
		cfg.MaxEventsPerPoll = 42
		cfg.EventDrivenScraping = true

		op, err := cfg.Build(component.TelemetrySettings{Logger: zap.NewNop()})
		require.NoError(t, err)
		input := op.(*Input)
		require.Zero(t, input.maxEventsPerPollCycle)
	})

	t.Run("feature gate forces zero", func(t *testing.T) {
		require.NoError(t, featuregate.GlobalRegistry().Set(gateID, true))
		t.Cleanup(func() {
			require.NoError(t, featuregate.GlobalRegistry().Set(gateID, false))
		})

		cfg := NewConfig()
		cfg.Channel = "Application"
		cfg.MaxEventsPerPoll = 42

		op, err := cfg.Build(component.TelemetrySettings{Logger: zap.NewNop()})
		require.NoError(t, err)
		input := op.(*Input)
		require.Zero(t, input.maxEventsPerPollCycle)
	})

	t.Run("polling mode preserves value", func(t *testing.T) {
		require.NoError(t, featuregate.GlobalRegistry().Set(gateID, false))

		cfg := NewConfig()
		cfg.Channel = "Application"
		cfg.MaxEventsPerPoll = 42

		op, err := cfg.Build(component.TelemetrySettings{Logger: zap.NewNop()})
		require.NoError(t, err)
		input := op.(*Input)
		require.Equal(t, 42, input.maxEventsPerPollCycle)
	})
}
