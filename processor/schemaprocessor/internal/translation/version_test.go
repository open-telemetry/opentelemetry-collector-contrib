// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		input    string
		err      error
		ident    *Version
	}{
		{
			scenario: "valid schema version",
			input:    "1.33.7",
			err:      nil,
			ident:    &Version{Major: 1, Minor: 33, Patch: 7},
		},
		{
			scenario: "schema version incomplete",
			input:    "1.0",
			err:      ErrInvalidVersion,
			ident:    nil,
		},
		{
			scenario: "schema version with non numerical value",
			input:    "v1.0.0",
			err:      strconv.ErrSyntax,
			ident:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ident, err := NewVersion(tc.input)

			assert.ErrorIs(t, err, tc.err, "MUST have the expected error")
			assert.Equal(t, tc.ident, ident, "MUST match the expected value")
			if ident != nil {
				assert.Equal(t, tc.input, ident.String(), "Stringer value MUST match input value")
			}
		})
	}
}

func TestParsingVersionFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		path     string
		ident    *Version
		err      error
	}{
		{scenario: "No path set", path: "", ident: nil, err: ErrInvalidVersion},
		{scenario: "no schema version defined", path: "foo/bar", ident: nil, err: ErrInvalidVersion},
		{scenario: "a valid path with schema version", path: "foo/bar/1.8.0", ident: &Version{Major: 1, Minor: 8, Patch: 0}, err: nil},
		{scenario: "a path with a trailing slash", path: "foo/bar/1.5.3/", ident: nil, err: ErrInvalidVersion},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ident, err := ReadVersionFromPath(tc.path)

			assert.ErrorIs(t, err, tc.err, "Must be the expected error when processor")
			assert.Equal(t, tc.ident, ident)
		})
	}
}

func TestVersionDifferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		a, b     Version
		diff     int
	}{
		{scenario: "equal values", a: Version{1, 0, 0}, b: Version{1, 0, 0}, diff: 0},
		{scenario: "greater than Major", a: Version{2, 6, 4}, b: Version{1, 8, 12}, diff: 1},
		{scenario: "greater than Minor", a: Version{2, 6, 4}, b: Version{2, 4, 12}, diff: 1},
		{scenario: "greater than Patch", a: Version{2, 6, 4}, b: Version{2, 6, 3}, diff: 1},
		{scenario: "less than Major", b: Version{2, 6, 4}, a: Version{1, 8, 12}, diff: -1},
		{scenario: "less than Minor", b: Version{2, 6, 4}, a: Version{2, 4, 12}, diff: -1},
		{scenario: "less than Patch", b: Version{2, 6, 4}, a: Version{2, 6, 3}, diff: -1},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			assert.Equal(t, tc.diff, tc.a.Compare(&tc.b), "Must match the expected diff")
		})
	}
}

func TestVersionOperators(t *testing.T) {
	t.Parallel()

	var (
		a = &Version{1, 4, 6}
		b = &Version{2, 3, 1}
	)

	assert.True(t, a.LessThan(b))
	assert.True(t, b.GreaterThan(a))
	assert.False(t, b.Equal(a))
}

// ver is declared here so that the compiler doesn't optimize out the result
var ver *Version

func BenchmarkParsingVersion(b *testing.B) {
	b.ReportAllocs()

	b.StopTimer()
	for i := 0; i < b.N; i++ {
		var err error
		b.StartTimer()
		ver, err = NewVersion("1.16.9")

		b.StopTimer()
		assert.NoError(b, err, "Must not error when parsing version")
	}
}
