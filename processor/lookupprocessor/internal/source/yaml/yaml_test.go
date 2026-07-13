// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
			config:  &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: "/path/to/file.yaml"}},
			wantErr: false,
		},
		{
			name:    "valid reload interval",
			config:  &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: "/path/to/file.yaml", ReloadInterval: 5 * time.Minute}},
			wantErr: false,
		},
		{
			name:    "negative reload interval",
			config:  &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: "/path/to/file.yaml", ReloadInterval: -1}},
			wantErr: true,
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
	cfg := &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: yamlPath}}

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

func TestYAMLSourceMapLookup(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "users.yaml")

	yamlContent := `
user001:
  name: "Alice Johnson"
  email: "alice@example.com"
  role: "admin"
user002:
  name: "Bob Smith"
  email: "bob@example.com"
  role: "viewer"
`
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0o600)
	require.NoError(t, err)

	factory := NewFactory()
	cfg := &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: yamlPath}}

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, source.Start(t.Context(), host))
	defer func() { _ = source.Shutdown(t.Context()) }()

	// Lookup should return map[string]any for nested YAML
	val, found, err := source.Lookup(t.Context(), "user001")
	require.NoError(t, err)
	require.True(t, found)

	m, ok := val.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", val)
	assert.Equal(t, "Alice Johnson", m["name"])
	assert.Equal(t, "alice@example.com", m["email"])
	assert.Equal(t, "admin", m["role"])

	// Second entry
	val, found, err = source.Lookup(t.Context(), "user002")
	require.NoError(t, err)
	require.True(t, found)

	m, ok = val.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bob Smith", m["name"])
	assert.Equal(t, "viewer", m["role"])

	// Not found
	_, found, err = source.Lookup(t.Context(), "user999")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestYAMLSourceFileNotFound(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: "/nonexistent/path/to/file.yaml"}}

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	err = source.Start(t.Context(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestYAMLSourceInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	err := os.WriteFile(yamlPath, []byte("not: valid: yaml: content: ["), 0o600)
	require.NoError(t, err)

	factory := NewFactory()
	cfg := &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: yamlPath}}

	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	err = source.Start(t.Context(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse file")
}

func TestYAMLSourceReload(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "mappings.yaml")

	require.NoError(t, os.WriteFile(yamlPath, []byte("store1010: open_store\n"), 0o600))

	factory := NewFactory()
	cfg := &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: yamlPath, ReloadInterval: 20 * time.Millisecond}}
	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	require.NoError(t, source.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, source.Shutdown(t.Context())) }()

	val, found, err := source.Lookup(t.Context(), "store1010")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "open_store", val)

	require.NoError(t, os.WriteFile(yamlPath, []byte("store1010: closed_store\n"), 0o600))

	require.Eventually(t, func() bool {
		v, ok, lookupErr := source.Lookup(t.Context(), "store1010")
		return lookupErr == nil && ok && v == "closed_store"
	}, 2*time.Second, 10*time.Millisecond)
}

func TestYAMLSourceReloadKeepsLastGoodOnError(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "mappings.yaml")

	require.NoError(t, os.WriteFile(yamlPath, []byte("store1010: closed_store\n"), 0o600))

	factory := NewFactory()
	cfg := &Config{FileSourceConfig: lookupsource.FileSourceConfig{Path: yamlPath, ReloadInterval: 20 * time.Millisecond}}
	settings := lookupsource.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	require.NoError(t, source.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, source.Shutdown(t.Context())) }()

	require.NoError(t, os.WriteFile(yamlPath, []byte("not: valid: yaml: ["), 0o600))

	require.Never(t, func() bool {
		v, ok, lookupErr := source.Lookup(t.Context(), "store1010")
		return lookupErr != nil || !ok || v != "closed_store"
	}, 200*time.Millisecond, 20*time.Millisecond)
}
