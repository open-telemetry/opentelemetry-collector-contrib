// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
)

var creationSet = receivertest.NewNopSettings(metadata.Type)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tReceiver, err := factory.CreateTraces(t.Context(), creationSet, cfg, consumertest.NewNop())
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tReceiver)

	mReceiver, err := factory.CreateMetrics(t.Context(), creationSet, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)

	tLogs, err := factory.CreateLogs(t.Context(), creationSet, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tLogs)
}

func TestCreateReceiver_ScraperKeyConfigError(t *testing.T) {
	const errorKey string = "error"

	factory := NewFactory()
	cfg := &Config{Scrapers: map[component.Type]component.Config{component.MustNewType(errorKey): &mockConfig{}}}

	_, err := factory.CreateMetrics(t.Context(), creationSet, cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("host metrics scraper factory not found for key: %q", errorKey))
}

func TestCreateMetrics_FeatureGateSystemConventions(t *testing.T) {
	tests := []struct {
		name              string
		receiverLevelGate string
		scraperLevelGate  string
		errorMsg          string
	}{
		{
			name:              "Emit V1 SystemConventions",
			receiverLevelGate: "receiver.hostmetrics.EmitV1SystemConventions",
			scraperLevelGate:  "scraper.process.EmitV1SystemConventions",
			errorMsg:          "receiver-level EmitV1 gate should enable the process scraper gate",
		},
		{
			name:              "Dont Emit Legacy SystemConventions",
			receiverLevelGate: "receiver.hostmetrics.DontEmitV0SystemConventions",
			scraperLevelGate:  "scraper.process.DontEmitV0SystemConventions",
			errorMsg:          "receiver-level DontEmitV0 gate should enable the process scraper gate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := featuregate.GlobalRegistry()

			require.NoError(t, reg.Set(tt.receiverLevelGate, true))
			t.Cleanup(func() {
				require.NoError(t, reg.Set(tt.receiverLevelGate, false))
				require.NoError(t, reg.Set(tt.scraperLevelGate, false))
			})

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			mReceiver, err := factory.CreateMetrics(t.Context(), creationSet, cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NotNil(t, mReceiver)

			assert.True(t, getEnabledFeatureGate(t, tt.scraperLevelGate), tt.errorMsg)
		})
	}
}

func getEnabledFeatureGate(t *testing.T, id string) bool {
	t.Helper()
	found := false
	enabled := false
	featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
		if g.ID() == id {
			found = true
			enabled = g.IsEnabled()
		}
	})
	require.True(t, found, "feature gate %q not registered", id)
	return enabled
}
