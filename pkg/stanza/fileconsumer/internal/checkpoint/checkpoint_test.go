// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestLoadNothing(t *testing.T) {
	reloaded, err := Load(t.Context(), testutil.NewUnscopedMockPersister(), zap.NewNop())
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
		}), zap.NewNop())
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
			reloaded, err := Load(t.Context(), p, zap.NewNop())
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
	reloaded, err := Load(t.Context(), p, zap.NewNop())
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

// TestProtobufEncodingDifferentLogSizes tests Save/Load with protobuf encoding
func TestProtobufEncodingDifferentLogSizes(t *testing.T) {
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
			reloaded, err := Load(t.Context(), p, zap.NewNop())
			require.NoError(t, err)
			require.Equal(t, tc.rmds, reloaded)
		})
	}
}

// TestLoadLegacyJSON ensures JSON checkpoints written by older versions are still loaded
// and are rewritten as protobuf on the next save
func TestLoadLegacyJSON(t *testing.T) {
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

	p := testutil.NewUnscopedMockPersister()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	require.NoError(t, enc.Encode(len(testData)))
	for _, rmd := range testData {
		require.NoError(t, enc.Encode(rmd))
	}
	require.NoError(t, p.Set(t.Context(), knownFilesKey, buf.Bytes()))

	reloaded, err := Load(t.Context(), p, zap.NewNop())
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)

	require.NoError(t, Save(t.Context(), p, reloaded))
	encoded, err := p.Get(t.Context(), knownFilesKey)
	require.NoError(t, err)
	_, err = tryLoadProtobuf(encoded)
	require.NoError(t, err, "checkpoint should be rewritten as protobuf")

	reloaded, err = Load(t.Context(), p, zap.NewNop())
	require.NoError(t, err)
	require.Equal(t, testData, reloaded)
}

// TestMigrateHeaderAttributesWithProtobuf ensures deprecated header attributes
// migration still works with protobuf encoding
func TestMigrateHeaderAttributesWithProtobuf(t *testing.T) {
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
	reloaded, err := Load(t.Context(), p, zap.NewNop())
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
			HeaderFinalized:         true,
			FileType:                ".gz",
			DecompressedBytesToSkip: 123,
		},
	}

	p := testutil.NewUnscopedMockPersister()
	require.NoError(t, Save(t.Context(), p, testData))

	reloaded, err := Load(t.Context(), p, zap.NewNop())
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
