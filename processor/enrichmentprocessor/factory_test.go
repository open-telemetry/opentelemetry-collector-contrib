// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor/internal/metadata"
)

func TestFactory(t *testing.T) {
	factory := NewFactory()

	t.Run("factory_creation", func(t *testing.T) {
		assert.NotNil(t, factory)
		assert.Equal(t, metadata.Type, factory.Type())
	})

	t.Run("default_config", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		assert.NotNil(t, cfg)

		enrichmentCfg, ok := cfg.(*Config)
		require.True(t, ok)

		// Check default values
		assert.Empty(t, enrichmentCfg.DataSources)
		assert.Empty(t, enrichmentCfg.EnrichmentRules)
	})
}

func TestCreateProcessors_EmptyConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := processortest.NewNopSettings(metadata.Type)

	// Test creating processors with empty config
	processor, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, processor)
}

func TestCreateProcessors_ValidConfig(t *testing.T) {
	factory := NewFactory()

	// Create test file
	tempDir := t.TempDir()
	jsonFile := filepath.Join(tempDir, "test.json")
	jsonData := []map[string]any{
		{"name": "test-service", "owner": "test-team"},
	}
	data, _ := json.Marshal(jsonData)
	_ = os.WriteFile(jsonFile, data, 0o600)

	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "services",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:            jsonFile,
					Format:          "json",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:               "service-enrichment",
				DataSource:         "services",
				LookupAttributeKey: "service.name",
				LookupField:        "name",
				Mappings: []FieldMapping{
					{SourceField: "owner", TargetAttribute: "team.name"},
				},
			},
		},
	}

	set := processortest.NewNopSettings(metadata.Type)
	processor, err := factory.CreateLogs(t.Context(), set, config, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, processor)

	// Cleanup
	if shutdownable, ok := processor.(interface{ Shutdown(context.Context) error }); ok {
		_ = shutdownable.Shutdown(t.Context())
	}
}
