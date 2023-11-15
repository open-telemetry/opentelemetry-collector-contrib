// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package checkpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestLoadNothing(t *testing.T) {
	reloaded, err := Load(context.Background(), testutil.NewUnscopedMockPersister())
	assert.NoError(t, err)
	assert.Equal(t, []*reader.Metadata{}, reloaded)
}

func TestSaveErr(t *testing.T) {
	assert.Error(t, Save(context.Background(),
		testutil.NewErrPersister(map[string]error{
			"knownFiles": assert.AnError,
		}), []*reader.Metadata{}))
}

func TestLoadErr(t *testing.T) {
	_, err := Load(context.Background(),
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
					Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("foo")},
					Offset:      3,
				},
			},
		},
		{
			"two",
			[]*reader.Metadata{
				{
					Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("foo")},
					Offset:      3,
				},
				{
					Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("barrrr")},
					Offset:      6,
				},
			},
		},
		{
			"other_fields",
			[]*reader.Metadata{
				{
					Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("foo")},
					Offset:      3,
					FileAttributes: map[string]any{
						"hello": "world",
					},
				},
				{
					Fingerprint:     &fingerprint.Fingerprint{FirstBytes: []byte("barrrr")},
					Offset:          6,
					HeaderFinalized: true,
				},
				{
					Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("ab")},
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
			assert.NoError(t, Save(context.Background(), p, tc.rmds))
			reloaded, err := Load(context.Background(), p)
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
			Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("foo")},
			Offset:      3,
			FileAttributes: map[string]any{
				"HeaderAttributes": map[string]any{
					"hello": "world",
				},
			},
		},
	})
	reloaded, err := Load(context.Background(), p)
	assert.NoError(t, err)
	assert.Equal(t, []*reader.Metadata{
		{
			Fingerprint: &fingerprint.Fingerprint{FirstBytes: []byte("foo")},
			Offset:      3,
			FileAttributes: map[string]any{
				"hello": "world",
			},
		},
	}, reloaded)

}

func saveDeprecated(t *testing.T, persister operator.Persister, dep *deprecatedMetadata) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	require.NoError(t, enc.Encode(1))
	require.NoError(t, enc.Encode(dep))
	require.NoError(t, persister.Set(context.Background(), knownFilesKey, buf.Bytes()))
}
