// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package networkscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)

	networkCfg := cfg.(*Config)
	assert.True(t, networkCfg.Connections.ExcludeLocalhost)
	assert.True(t, networkCfg.Connections.ExcludeListenPorts)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)

	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}

func TestCreateMetrics_Error(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "invalid interface include filter",
			cfg:  &Config{Include: MatchConfig{Interfaces: []string{""}}},
		},
		{
			name: "invalid process include filter",
			cfg:  &Config{Connections: ConnectionConfig{IncludeProcesses: ProcessMatchConfig{Names: []string{""}}}},
		},
		{
			name: "invalid process exclude filter",
			cfg:  &Config{Connections: ConnectionConfig{ExcludeProcesses: ProcessMatchConfig{Names: []string{""}}}},
		},
	}

	factory := NewFactory()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := factory.CreateMetrics(t.Context(), scrapertest.NewNopSettings(metadata.Type), test.cfg)
			assert.Error(t, err)
		})
	}
}
