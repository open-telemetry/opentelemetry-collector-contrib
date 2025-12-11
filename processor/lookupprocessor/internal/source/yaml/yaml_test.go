// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "yaml", factory.Type())
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "empty path",
			config:  &Config{},
			wantErr: true,
		},
		{
			name:    "valid path",
			config:  &Config{Path: "/path/to/file.yaml"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestYAMLSourceLookup(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "mappings.yaml")

	yamlContent := `
user001: "Alice Johnson"
user002: "Bob Smith"
user003: "Charlie Brown"
svc-frontend: "Frontend Web App"
svc-backend: "Backend API Service"
numeric_key: 42
bool_key: true
`
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0o600)
	require.NoError(t, err)

	factory := NewFactory()
	cfg := &Config{Path: yamlPath}

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)
	require.NotNil(t, source)

	// Start the source to load the file
	host := componenttest.NewNopHost()
	require.NoError(t, source.Start(t.Context(), host))

	// Test lookups
	tests := []struct {
		key      string
		expected any
		found    bool
	}{
		{"user001", "Alice Johnson", true},
		{"user002", "Bob Smith", true},
		{"user003", "Charlie Brown", true},
		{"svc-frontend", "Frontend Web App", true},
		{"svc-backend", "Backend API Service", true},
		{"numeric_key", 42, true},
		{"bool_key", true, true},
		{"nonexistent", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			val, found, err := source.Lookup(t.Context(), tt.key)
			require.NoError(t, err)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected, val)
			}
		})
	}

	// Shutdown
	require.NoError(t, source.Shutdown(t.Context()))
}

func TestYAMLSourceFileNotFound(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{Path: "/nonexistent/path/to/file.yaml"}

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	err = source.Start(t.Context(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read YAML file")
}

func TestYAMLSourceInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	err := os.WriteFile(yamlPath, []byte("not: valid: yaml: content: ["), 0o600)
	require.NoError(t, err)

	factory := NewFactory()
	cfg := &Config{Path: yamlPath}

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	err = source.Start(t.Context(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML file")
}
