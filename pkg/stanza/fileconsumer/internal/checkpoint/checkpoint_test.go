// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestLoadNothing(t *testing.T) {
	reloaded, err := Load(t.Context(), testutil.NewUnscopedMockPersister())
	assert.NoError(t, err)
	assert.Equal(t, []*reader.Metadata{}, reloaded)
}

func TestSaveErr(t *testing.T) {
	assert.Error(t, Save(t.Context(),
		testutil.NewErrPersister(map[string]error{
			"knownFiles": assert.AnError,
		}), []*reader.Metadata{}))
}

func TestLoadErr(t *testing.T) {
	_, err := Load(t.Context(),
		testutil.NewErrPersister(map[string]error{
			"knownFiles": assert.AnError,
		}))
	assert.Error(t, err)
}

func TestNopEncodingDifferentLogSizes(t *testing.T) {
	testCases := []struct {
		name string
		rmds []*reader.Metadata
	}{
		{
			"empty",
			[]*reader.Metadata{},
		},
		{
			"one",
			[]*reader.Metadata{
				{
					FileAttributes: make(map[string]any),
					Fingerprint:    fingerprint.New([]byte("foo")),
					Offset:         3,
				},
			},
		},
		{
			"two",
			[]*reader.Metadata{
				{
					FileAttributes: make(map[string]any),
					Fingerprint:    fingerprint.New([]byte("foo")),
					Offset:         3,
				},
				{
					FileAttributes: make(map[string]any),
					Fingerprint:    fingerprint.New([]byte("barrrr")),
					Offset:         6,
				},
			},
		},
		{
			"other_fields",
			[]*reader.Metadata{
				{
					Fingerprint: fingerprint.New([]byte("foo")),
					Offset:      3,
					FileAttributes: map[string]any{
						"hello": "world",
					},
				},
				{
					FileAttributes:  make(map[string]any),
					Fingerprint:     fingerprint.New([]byte("barrrr")),
					Offset:          6,
					HeaderFinalized: true,
				},
				{
					Fingerprint: fingerprint.New([]byte("ab")),
					Offset:      2,
					FileAttributes: map[string]any{
						"hello2": "world2",
					},
					HeaderFinalized: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.NewUnscopedMockPersister()
			assert.NoError(t, Save(t.Context(), p, tc.rmds))
			reloaded, err := Load(t.Context(), p)
			assert.NoError(t, err)
			assert.Equal(t, tc.rmds, reloaded)
		})
	}
}

type deprecatedMetadata struct {
	reader.Metadata
	HeaderAttributes map[string]any
}

func TestMigrateHeaderAttributes(t *testing.T) {
	p := testutil.NewUnscopedMockPersister()
	saveDeprecated(t, p, &deprecatedMetadata{
		Metadata: reader.Metadata{
			Fingerprint: fingerprint.New([]byte("foo")),
			Offset:      3,
			FileAttributes: map[string]any{
				"HeaderAttributes": map[string]any{
					"hello": "world",
				},
			},
		},
	})
	reloaded, err := Load(t.Context(), p)
	assert.NoError(t, err)
	assert.Equal(t, []*reader.Metadata{
		{
			Fingerprint: fingerprint.New([]byte("foo")),
			Offset:      3,
			FileAttributes: map[string]any{
				"hello": "world",
			},
		},
	}, reloaded)
}

// TestProtobufEncodingDifferentLogSizes tests Save/Load with protobuf encoding enabled
func TestProtobufEncodingDifferentLogSizes(t *testing.T) {
	setProtobufEncoding(t, true)

	testCases := []struct {
		name string
		rmds []*reader.Metadata
	}{
		{
			"empty",
			[]*reader.Metadata{},
		},
		{
			"one",
			[]*reader.Metadata{
				{
					FileAttributes: make(map[string]any),
					Fingerprint:    fingerprint.New([]byte("foo")),
					Offset:         3,
				},
			},
		},
		{
			"two",
			[]*reader.Metadata{
				{
					FileAttributes: make(map[string]any),
					Fingerprint:    fingerprint.New([]byte("foo")),
					Offset:         3,
				},
				{
					FileAttributes: make(map[string]any),
					Fingerprint:    fingerprint.New([]byte("barrrr")),
					Offset:         6,
				},
			},
		},
		{
			"other_fields",
			[]*reader.Metadata{
				{
					Fingerprint: fingerprint.New([]byte("foo")),
					Offset:      3,
					FileAttributes: map[string]any{
						"hello": "world",
					},
				},
				{
					FileAttributes:  make(map[string]any),
					Fingerprint:     fingerprint.New([]byte("barrrr")),
					Offset:          6,
					HeaderFinalized: true,
				},
				{
					Fingerprint: fingerprint.New([]byte("ab")),
					Offset:      2,
					FileAttributes: map[string]any{
						"hello2": "world2",
					},
					HeaderFinalized: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.NewUnscopedMockPersister()
			require.NoError(t, Save(t.Context(), p, tc.rmds))
			reloaded, err := Load(t.Context(), p)
			require.NoError(t, err)
			require.Equal(t, tc.rmds, reloaded)
		})
	}
}

// TestCrossFormatCompatibility tests that data saved with one encoding can be loaded with another
func TestCrossFormatCompatibility(t *testing.T) {
	testData := []*reader.Metadata{
		{
			Fingerprint: fingerprint.New([]byte("test-file")),
			Offset:      1024,
			RecordNum:   42,
			FileAttributes: map[string]any{
				"log.file.name": "app.log",
				"host.name":     "server-001",
			},
			HeaderFinalized: true,
		},
	}

	t.Run("json_to_protobuf", func(t *testing.T) {
		p := testutil.NewUnscopedMockPersister()

		// Save with JSON (feature gate disabled)
		setProtobufEncoding(t, false)
		require.NoError(t, Save(t.Context(), p, testData))

		// Load with protobuf feature gate enabled (should fallback to JSON)
		setProtobufEncoding(t, true)
		reloaded, err := Load(t.Context(), p)
		require.NoError(t, err)
		require.Equal(t, testData, reloaded)
	})

	t.Run("protobuf_to_json", func(t *testing.T) {
		p := testutil.NewUnscopedMockPersister()

		// Save with protobuf (feature gate enabled)
		setProtobufEncoding(t, true)
		require.NoError(t, Save(t.Context(), p, testData))

		// Load with feature gate disabled (should try protobuf first, succeed)
		setProtobufEncoding(t, false)
		reloaded, err := Load(t.Context(), p)
		require.NoError(t, err)
		require.Equal(t, testData, reloaded)
	})
}

// TestFeatureGateToggle tests the scenario where feature gate is toggled on and off
func TestFeatureGateToggle(t *testing.T) {
	testData := []*reader.Metadata{
		{
			Fingerprint: fingerprint.New([]byte("toggle-test")),
			Offset:      2048,
			RecordNum:   100,
			FileAttributes: map[string]any{
				"environment": "production",
			},
		},
	}

	p := testutil.NewUnscopedMockPersister()

	// Step 1: Save with JSON (disabled)
	setProtobufEncoding(t, false)
	require.NoError(t, Save(t.Context(), p, testData))

	// Step 2: Load with JSON, verify
	reloaded, err := Load(t.Context(), p)
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)

	// Step 3: Enable protobuf, save
	setProtobufEncoding(t, true)
	require.NoError(t, Save(t.Context(), p, testData))

	// Step 4: Load with protobuf enabled, verify
	reloaded, err = Load(t.Context(), p)
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)

	// Step 5: Disable protobuf again
	setProtobufEncoding(t, false)
	reloaded, err = Load(t.Context(), p)
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)

	// Step 6: Save with JSON again (overwriting protobuf)
	require.NoError(t, Save(t.Context(), p, testData))

	// Step 7: Load with JSON, verify still works
	reloaded, err = Load(t.Context(), p)
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)
}

// TestMigrateHeaderAttributesWithProtobuf ensures deprecated header attributes
// migration still works when protobuf encoding is enabled
func TestMigrateHeaderAttributesWithProtobuf(t *testing.T) {
	setProtobufEncoding(t, true)

	p := testutil.NewUnscopedMockPersister()
	saveDeprecated(t, p, &deprecatedMetadata{
		Metadata: reader.Metadata{
			Fingerprint: fingerprint.New([]byte("foo")),
			Offset:      3,
			FileAttributes: map[string]any{
				"HeaderAttributes": map[string]any{
					"hello": "world",
				},
			},
		},
	})
	reloaded, err := Load(t.Context(), p)
	require.NoError(t, err)
	require.Equal(t, []*reader.Metadata{
		{
			Fingerprint: fingerprint.New([]byte("foo")),
			Offset:      3,
			FileAttributes: map[string]any{
				"hello": "world",
			},
		},
	}, reloaded)
}

// TestProtobufWithComplexMetadata tests protobuf encoding with all fields populated
func TestProtobufWithComplexMetadata(t *testing.T) {
	setProtobufEncoding(t, true)

	testData := []*reader.Metadata{
		{
			Fingerprint: fingerprint.New([]byte("complex-test-data")),
			Offset:      4096,
			RecordNum:   999,
			FileAttributes: map[string]any{
				"log.file.name": "complex.log",
				"log.file.path": "/var/log/complex.log",
				"host.name":     "test-server",
				"service.name":  "test-service",
				"environment":   "staging",
				"nested_attr":   map[string]any{"key": "value"},
				"numeric_attr":  float64(42), // JSON unmarshaling converts to float64
				"boolean_attr":  true,
				"array_attr":    []any{"item1", "item2"},
			},
			HeaderFinalized: true,
			FileType:        ".log",
		},
	}

	p := testutil.NewUnscopedMockPersister()
	require.NoError(t, Save(t.Context(), p, testData))

	reloaded, err := Load(t.Context(), p)
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)

	// Verify the data was actually saved as protobuf by checking it can't be decoded as JSON
	encoded, err := p.Get(t.Context(), knownFilesKey)
	require.NoError(t, err)

	// Try to decode as JSON - should fail because it's protobuf
	var jsonTest any
	err = json.Unmarshal(encoded, &jsonTest)
	require.Error(t, err, "Data should be protobuf, not JSON")
}

func saveDeprecated(t *testing.T, persister operator.Persister, dep *deprecatedMetadata) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	require.NoError(t, enc.Encode(1))
	require.NoError(t, enc.Encode(dep))
	require.NoError(t, persister.Set(t.Context(), knownFilesKey, buf.Bytes()))
}

// setProtobufEncoding enables or disables protobuf checkpoint encoding for tests
// It saves the current state and restores it after the test completes
// This is exported so it can be used in parent package benchmarks
func setProtobufEncoding(tb testing.TB, value bool) {
	currentState := ProtobufEncodingFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(ProtobufEncodingFeatureGate.ID(), value))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(ProtobufEncodingFeatureGate.ID(), currentState))
	})
}
