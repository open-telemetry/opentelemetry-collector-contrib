// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileset // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

type test[T Matchable] struct {
	name    string
	fileset *Fileset[T]
	ops     []func(t *testing.T, fileset *Fileset[T])
}

func (t *test[T]) init() {
	t.fileset = New[T](10)
}

func push[T Matchable](ele ...T) func(t *testing.T, fileset *Fileset[T]) {
	return func(t *testing.T, fileset *Fileset[T]) {
		pr := fileset.Len()
		fileset.Add(ele...)
		require.Equal(t, pr+len(ele), fileset.Len())
	}
}

func pop[T Matchable](expectedErr error, expectedElement T) func(t *testing.T, fileset *Fileset[T]) {
	return func(t *testing.T, fileset *Fileset[T]) {
		pr := fileset.Len()
		el, err := fileset.Pop()
		if expectedErr == nil {
			require.NoError(t, err)
			require.Equal(t, expectedElement, el)
			require.Equal(t, pr-1, fileset.Len())
		} else {
			require.ErrorIs(t, err, expectedErr)
		}
	}
}

func match[T Matchable](ele T, expect bool) func(t *testing.T, fileset *Fileset[T]) {
	return func(t *testing.T, fileset *Fileset[T]) {
		pr := fileset.Len()
		r := fileset.Match(ele.GetFingerprint(), StartsWith)
		if expect {
			require.NotNil(t, r)
			require.Equal(t, pr-1, fileset.Len())
		} else {
			require.Nil(t, r)
			require.Equal(t, pr, fileset.Len())
		}
	}
}

func newReader(bytes []byte) *reader.Reader {
	return &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New(bytes),
		},
	}
}

func TestFilesetReader(t *testing.T) {
	testCases := []test[*reader.Reader]{
		{
			name: "test_match_push_reset",
			ops: []func(t *testing.T, fileset *Fileset[*reader.Reader]){
				push(newReader([]byte("ABCDEF")), newReader([]byte("QWERT"))),

				// match() removes the matched item and returns it
				match(newReader([]byte("ABCDEFGHI")), true),
				match(newReader([]byte("ABCDEFGHI")), false),

				push(newReader([]byte("XYZ"))),
				match(newReader([]byte("ABCDEF")), false),
				match(newReader([]byte("QWERT")), true), // should still be present
				match(newReader([]byte("XYZabc")), true),
				pop(errFilesetEmpty, newReader([]byte(""))),
			},
		},
		{
			name: "test_pop",
			ops: []func(t *testing.T, fileset *Fileset[*reader.Reader]){
				push(newReader([]byte("ABCDEF")), newReader([]byte("QWERT"))),
				pop(nil, newReader([]byte("ABCDEF"))),
				pop(nil, newReader([]byte("QWERT"))),
				pop(errFilesetEmpty, newReader([]byte(""))),

				push(newReader([]byte("XYZ"))),
				pop(nil, newReader([]byte("XYZ"))),
				pop(errFilesetEmpty, newReader([]byte(""))),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.init()
			for _, op := range tc.ops {
				op(t, tc.fileset)
			}
		})
	}
}
