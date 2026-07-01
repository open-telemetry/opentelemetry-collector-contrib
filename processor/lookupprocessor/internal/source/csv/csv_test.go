// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package csv

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

func intPtr(i int) *int { return &i }

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "csv", factory.Type())
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr string
	}{
		{
			name:    "empty path",
			config:  &Config{KeyColumn: "id", HasHeader: true},
			wantErr: "path is required",
		},
		{
			name:   "valid by name",
			config: &Config{Path: "f.csv", HasHeader: true, KeyColumn: "id"},
		},
		{
			name:   "valid headerless by index",
			config: &Config{Path: "f.csv", HasHeader: false, KeyColumnIndex: intPtr(0)},
		},
		{
			name:    "no key selector",
			config:  &Config{Path: "f.csv", HasHeader: true},
			wantErr: "one of key_column or key_column_index is required",
		},
		{
			name:    "both key selectors",
			config:  &Config{Path: "f.csv", HasHeader: true, KeyColumn: "id", KeyColumnIndex: intPtr(0)},
			wantErr: "only one of key_column or key_column_index",
		},
		{
			name:    "name selector without header",
			config:  &Config{Path: "f.csv", HasHeader: false, KeyColumn: "id"},
			wantErr: "requires has_header: true",
		},
		{
			name:    "negative index",
			config:  &Config{Path: "f.csv", HasHeader: false, KeyColumnIndex: intPtr(-1)},
			wantErr: "key_column_index must not be negative",
		},
		{
			name:    "multi-char delimiter",
			config:  &Config{Path: "f.csv", HasHeader: true, KeyColumn: "id", Delimiter: "||"},
			wantErr: "delimiter must be a single character",
		},
		{
			name:    "negative reload interval",
			config:  &Config{Path: "f.csv", HasHeader: true, KeyColumn: "id", ReloadInterval: -1},
			wantErr: "reload_interval must not be negative",
		},
		{
			name:    "both value selectors",
			config:  &Config{Path: "f.csv", HasHeader: true, KeyColumn: "id", ValueColumn: "v", ValueColumnIndex: intPtr(1)},
			wantErr: "only one of value_column or value_column_index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newSource(t *testing.T, cfg *Config) lookupsource.Source {
	t.Helper()
	require.NoError(t, cfg.Validate())
	factory := NewFactory()
	settings := lookupsource.CreateSettings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)
	require.NoError(t, source.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, source.Shutdown(t.Context())) })
	return source
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lookup.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestHeaderedScalarLookup(t *testing.T) {
	path := writeFile(t, "store_id,store_state\n1010,closed_store\n1011,open_store\n")
	source := newSource(t, &Config{
		Path: path, HasHeader: true, Delimiter: ",",
		KeyColumn: "store_id", ValueColumn: "store_state",
	})

	val, found, err := source.Lookup(t.Context(), "1010")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "closed_store", val)

	_, found, err = source.Lookup(t.Context(), "9999")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestHeaderlessByIndex(t *testing.T) {
	path := writeFile(t, "1010,closed_store\n1011,open_store\n")
	source := newSource(t, &Config{
		Path: path, HasHeader: false, Delimiter: ",",
		KeyColumnIndex: intPtr(0), ValueColumnIndex: intPtr(1),
	})

	val, found, err := source.Lookup(t.Context(), "1010")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "closed_store", val)
}

func TestHeaderedMapLookup(t *testing.T) {
	path := writeFile(t, "store_id,store_state,region\n1010,closed_store,NL\n")
	source := newSource(t, &Config{
		Path: path, HasHeader: true, Delimiter: ",", KeyColumn: "store_id",
	})

	val, found, err := source.Lookup(t.Context(), "1010")
	require.NoError(t, err)
	require.True(t, found)
	m, ok := val.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", val)
	assert.Equal(t, "closed_store", m["store_state"])
	assert.Equal(t, "NL", m["region"])
}

func TestHeaderlessMapLookupByIndexKeys(t *testing.T) {
	path := writeFile(t, "1010,closed_store,NL\n")
	source := newSource(t, &Config{
		Path: path, HasHeader: false, Delimiter: ",", KeyColumnIndex: intPtr(0),
	})

	val, found, err := source.Lookup(t.Context(), "1010")
	require.NoError(t, err)
	require.True(t, found)
	m, ok := val.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "closed_store", m["1"])
	assert.Equal(t, "NL", m["2"])
}

func TestSemicolonDelimiter(t *testing.T) {
	path := writeFile(t, "store_id;store_state\n1010;closed_store\n")
	source := newSource(t, &Config{
		Path: path, HasHeader: true, Delimiter: ";",
		KeyColumn: "store_id", ValueColumn: "store_state",
	})

	val, found, err := source.Lookup(t.Context(), "1010")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "closed_store", val)
}

func TestUnknownColumnNameFailsStart(t *testing.T) {
	path := writeFile(t, "store_id,store_state\n1010,closed_store\n")
	cfg := &Config{Path: path, HasHeader: true, KeyColumn: "does_not_exist"}
	require.NoError(t, cfg.Validate())

	factory := NewFactory()
	settings := lookupsource.CreateSettings{TelemetrySettings: componenttest.NewNopTelemetrySettings()}
	source, err := factory.CreateSource(t.Context(), settings, cfg)
	require.NoError(t, err)

	err = source.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `key column "does_not_exist" not found`)
}

func TestReloadPicksUpChanges(t *testing.T) {
	path := writeFile(t, "store_id,store_state\n1010,open_store\n")
	source := newSource(t, &Config{
		Path: path, HasHeader: true, KeyColumn: "store_id", ValueColumn: "store_state",
		ReloadInterval: 20 * time.Millisecond,
	})

	val, _, err := source.Lookup(t.Context(), "1010")
	require.NoError(t, err)
	require.Equal(t, "open_store", val)

	require.NoError(t, os.WriteFile(path, []byte("store_id,store_state\n1010,closed_store\n"), 0o600))

	require.Eventually(t, func() bool {
		v, ok, lookupErr := source.Lookup(t.Context(), "1010")
		return lookupErr == nil && ok && v == "closed_store"
	}, 2*time.Second, 10*time.Millisecond)
}
