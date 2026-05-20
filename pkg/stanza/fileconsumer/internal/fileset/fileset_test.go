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
			require.NotNil(t, el)
			require.True(t, expectedElemet.GetFingerprint().Equal(el.GetFingerprint()))
			require.Equal(t, pr-1, fileset.Len())
		} else {
			require.ErrorIs(t, err, expectedErr)
		}
	}
}

func match[T Matchable](ele T, expect bool) func(t *testing.T, fileset *Fileset[T]) {
	return func(t *testing.T, fileset *Fileset[T]) {
		pr := fileset.Len()
		r := fileset.MatchStartsWith(ele.GetFingerprint())
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

func TestFilesetMatchEqual(t *testing.T) {
	fs := New[*reader.Reader](10)
	r1 := newReader([]byte("AAAAA"))
	r2 := newReader([]byte("BBBBB"))
	r3 := newReader([]byte("CCCCC"))

	fs.Add(r1, r2, r3)

	matched := fs.MatchEqual(fingerprint.New([]byte("BBBBB")))
	require.Equal(t, r2, matched)
	require.Equal(t, 2, fs.Len())

	matched = fs.MatchEqual(fingerprint.New([]byte("AAAAA")))
	require.Equal(t, r1, matched)
	require.Equal(t, 1, fs.Len())

	matched = fs.MatchEqual(fingerprint.New([]byte("missing")))
	require.Nil(t, matched)
	require.Equal(t, 1, fs.Len())
}

func TestFilesetReset(t *testing.T) {
	fs := New[*reader.Reader](10)
	fs.Add(newReader([]byte("ABC")), newReader([]byte("DEF")))
	require.Equal(t, 2, fs.Len())

	fs.Reset()
	require.Zero(t, fs.Len())

	fs.Add(newReader([]byte("GHI")))
	require.Equal(t, 1, fs.Len())
	require.NotNil(t, fs.MatchEqual(fingerprint.New([]byte("GHI"))))
}

func TestFilesetNilAndEmptyFingerprints(t *testing.T) {
	fs := New[*reader.Reader](10)

	nilReader := &reader.Reader{
		Metadata: &reader.Metadata{},
	}
	emptyReader := &reader.Reader{
		Metadata: &reader.Metadata{Fingerprint: fingerprint.New([]byte{})},
	}
	fs.Add(nilReader, emptyReader)

	require.Nil(t, fs.MatchEqual(nil))
	require.Nil(t, fs.MatchEqual(fingerprint.New([]byte{})))
	require.Equal(t, 2, fs.Len())
}

func TestFilesetStartsWithAndMultipleRemoves(t *testing.T) {
	fs := New[*reader.Reader](100)

	for i := range 50 {
		data := []byte{byte(i), byte(i + 1), byte(i + 2)}
		fs.Add(newReader(data))
	}

	require.NotNil(t, fs.MatchEqual(fingerprint.New([]byte{25, 26, 27})))
	require.NotNil(t, fs.MatchEqual(fingerprint.New([]byte{0, 1, 2})))
	require.Equal(t, 48, fs.Len())

	fs.Add(newReader([]byte("ABCDEF")))
	require.NotNil(t, fs.MatchStartsWith(fingerprint.New([]byte("ABCDEFGHIJ"))))
}

func TestFilesetReindexAfterFingerprintChange(t *testing.T) {
	fs := New[*reader.Reader](10)
	r := newReader([]byte("ABC"))
	fs.Add(r)

	r.Fingerprint = fingerprint.New([]byte("ABCDEFG"))
	fs.Reindex()

	matched := fs.MatchStartsWith(fingerprint.New([]byte("ABCDEFGH")))
	require.Equal(t, r, matched)
}
