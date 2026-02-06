// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dirprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestScheme(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	assert.Equal(t, "dir", p.Scheme())
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}

func TestRetrieve_InvalidScheme(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, err := p.Retrieve(t.Context(), "file:/some/path", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not supported by")
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_NonExistentDirectory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, err := p.Retrieve(t.Context(), "dir:./testdata/nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_NotADirectory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, err := p.Retrieve(t.Context(), "dir:./testdata/base/receivers.yaml", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory")
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_InvalidYAML(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, err := p.Retrieve(t.Context(), "dir:./testdata/invalid", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_EmptyDirectory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	retrieved, err := p.Retrieve(t.Context(), "dir:./testdata/empty", nil)
	require.NoError(t, err)

	conf, err := retrieved.AsConf()
	require.NoError(t, err)
	assert.Empty(t, conf.AllKeys())
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_BaseDirectory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	retrieved, err := p.Retrieve(t.Context(), "dir:./testdata/base", nil)
	require.NoError(t, err)

	conf, err := retrieved.AsConf()
	require.NoError(t, err)

	// Check that all files were merged
	assert.True(t, conf.IsSet("receivers"))
	assert.True(t, conf.IsSet("exporters"))
	assert.True(t, conf.IsSet("service"))

	// Verify specific values
	assert.Equal(t, "0.0.0.0:4317", conf.Get("receivers::otlp::protocols::grpc::endpoint"))
	assert.Equal(t, "detailed", conf.Get("exporters::debug::verbosity"))

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_NestedDirectory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	retrieved, err := p.Retrieve(t.Context(), "dir:./testdata/nested", nil)
	require.NoError(t, err)

	conf, err := retrieved.AsConf()
	require.NoError(t, err)

	// Check that files from subdirectory were also included
	assert.True(t, conf.IsSet("receivers"))
	assert.True(t, conf.IsSet("processors"))

	// Verify values from both levels
	assert.Equal(t, "0.0.0.0:4317", conf.Get("receivers::otlp::protocols::grpc::endpoint"))
	assert.Equal(t, "5s", conf.Get("processors::batch::timeout"))

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestRetrieve_OverrideOrder(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	retrieved, err := p.Retrieve(t.Context(), "dir:./testdata/override", nil)
	require.NoError(t, err)

	conf, err := retrieved.AsConf()
	require.NoError(t, err)

	// 02-override.yaml should override 01-base.yaml (alphabetical order)
	// so verbosity should be "detailed" not "basic"
	assert.Equal(t, "detailed", conf.Get("exporters::debug::verbosity"))

	// receivers from 01-base.yaml should still be present
	assert.True(t, conf.IsSet("receivers"))
	assert.Equal(t, "0.0.0.0:4317", conf.Get("receivers::otlp::protocols::grpc::endpoint"))

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestShutdown(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	err := p.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		dst      map[string]any
		src      map[string]any
		expected map[string]any
	}{
		{
			name:     "nil dst",
			dst:      nil,
			src:      map[string]any{"key": "value"},
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "empty src",
			dst:      map[string]any{"key": "value"},
			src:      map[string]any{},
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "simple override",
			dst:      map[string]any{"key": "old"},
			src:      map[string]any{"key": "new"},
			expected: map[string]any{"key": "new"},
		},
		{
			name:     "add new key",
			dst:      map[string]any{"key1": "value1"},
			src:      map[string]any{"key2": "value2"},
			expected: map[string]any{"key1": "value1", "key2": "value2"},
		},
		{
			name: "nested merge",
			dst: map[string]any{
				"outer": map[string]any{
					"inner1": "value1",
				},
			},
			src: map[string]any{
				"outer": map[string]any{
					"inner2": "value2",
				},
			},
			expected: map[string]any{
				"outer": map[string]any{
					"inner1": "value1",
					"inner2": "value2",
				},
			},
		},
		{
			name: "nested override",
			dst: map[string]any{
				"outer": map[string]any{
					"inner": "old",
				},
			},
			src: map[string]any{
				"outer": map[string]any{
					"inner": "new",
				},
			},
			expected: map[string]any{
				"outer": map[string]any{
					"inner": "new",
				},
			},
		},
		{
			name: "map to non-map override",
			dst: map[string]any{
				"key": map[string]any{"nested": "value"},
			},
			src: map[string]any{
				"key": "scalar",
			},
			expected: map[string]any{
				"key": "scalar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMaps(tt.dst, tt.src)
			assert.Equal(t, tt.expected, result)
		})
	}
}
