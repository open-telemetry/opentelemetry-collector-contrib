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

func pop[T Matchable](expectedErr error, expectedElemet T) func(t *testing.T, fileset *Fileset[T]) {
	return func(t *testing.T, fileset *Fileset[T]) {
		pr := fileset.Len()
		el, err := fileset.Pop()
		if expectedErr == nil {
			require.NoError(t, err)
			require.Equal(t, expectedElemet, el)
			require.Equal(t, pr-1, fileset.Len())
		} else {
			require.ErrorIs(t, err, expectedErr)
		}
	}
}

func match[T Matchable](ele T, expect bool) func(t *testing.T, fileset *Fileset[T]) {
	return func(t *testing.T, fileset *Fileset[T]) {
		pr := fileset.Len()
		r := fileset.Match(ele.GetFingerprint(), CompareModeStartsWith)
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
	fp := fingerprint.New(bytes)
	fp.Key()
	return &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fp,
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
				pop(nil, newReader([]byte("QWERT"))),
				pop(nil, newReader([]byte("ABCDEF"))),
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

func TestFilesetBucketCollisions(t *testing.T) {
	fs := New[*reader.Reader](10)

	r1 := newReader([]byte("AAAAA"))
	r2 := newReader([]byte("BBBBB"))
	r3 := newReader([]byte("CCCCC"))

	fs.Add(r1, r2, r3)
	require.Equal(t, 3, fs.Len())

	matched := fs.Match(fingerprint.New([]byte("BBBBB")), CompareModeEqual)
	require.NotNil(t, matched)
	require.Equal(t, 2, fs.Len())

	matched = fs.Match(fingerprint.New([]byte("AAAAA")), CompareModeEqual)
	require.NotNil(t, matched)
	require.Equal(t, 1, fs.Len())
}

func TestFilesetReset(t *testing.T) {
	fs := New[*reader.Reader](10)

	fs.Add(newReader([]byte("ABC")), newReader([]byte("DEF")))
	require.Equal(t, 2, fs.Len())

	fs.Reset()
	require.Equal(t, 0, fs.Len())

	fs.Add(newReader([]byte("GHI")))
	require.Equal(t, 1, fs.Len())

	matched := fs.Match(fingerprint.New([]byte("GHI")), CompareModeEqual)
	require.NotNil(t, matched)
}

func TestFilesetNilFingerprints(t *testing.T) {
	fs := New[*reader.Reader](10)

	r := &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: nil,
		},
	}

	fs.Add(r)
	require.Equal(t, 1, fs.Len())

	matched := fs.Match(nil, CompareModeEqual)
	require.Nil(t, matched)

	popped, err := fs.Pop()
	require.NoError(t, err)
	require.Equal(t, r, popped)
}

func TestFilesetEmptyFingerprints(t *testing.T) {
	fs := New[*reader.Reader](10)

	r := &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New([]byte{}),
		},
	}

	fs.Add(r)
	require.Equal(t, 1, fs.Len())

	matched := fs.Match(fingerprint.New([]byte{}), CompareModeEqual)
	require.Nil(t, matched)
}

func TestFilesetCompareModes(t *testing.T) {
	fs := New[*reader.Reader](10)

	r1 := newReader([]byte("ABCDEF"))
	r2 := newReader([]byte("XYZ"))
	fs.Add(r1, r2)

	matched := fs.Match(fingerprint.New([]byte("ABCDEF")), CompareModeEqual)
	require.NotNil(t, matched)
	require.Equal(t, 1, fs.Len())

	fs.Add(r1)

	matched = fs.Match(fingerprint.New([]byte("ABCDEFGHIJ")), CompareModeStartsWith)
	require.NotNil(t, matched)
	require.Equal(t, 1, fs.Len())
}

func TestFilesetMultipleRemoves(t *testing.T) {
	fs := New[*reader.Reader](100)

	for i := range 50 {
		data := []byte{byte(i), byte(i + 1), byte(i + 2)}
		fs.Add(newReader(data))
	}
	require.Equal(t, 50, fs.Len())

	matched := fs.Match(fingerprint.New([]byte{25, 26, 27}), CompareModeEqual)
	require.NotNil(t, matched)
	require.Equal(t, 49, fs.Len())

	matched = fs.Match(fingerprint.New([]byte{0, 1, 2}), CompareModeEqual)
	require.NotNil(t, matched)
	require.Equal(t, 48, fs.Len())
}
