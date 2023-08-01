// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringMatcher(t *testing.T) {
	for _, tc := range []struct {
		name        string
		items       []string
		input       string
		shouldMatch bool
	}{
		{
			name:        "no items",
			items:       []string{},
			input:       "process_",
			shouldMatch: false,
		},
		{
			name: "matching exclude",
			items: []string{
				"!app",
			},
			input:       "app",
			shouldMatch: false,
		},
		{
			name: "unmatched exclude",
			items: []string{
				"!app",
			},
			input:       "something",
			shouldMatch: true,
		},
		{
			name: "single blob",
			items: []string{
				"pr*s_",
			},
			input:       "process_",
			shouldMatch: true,
		},
		{
			name: "unmatched blob exclude",
			items: []string{
				"!pr*s_",
			},
			input:       "other",
			shouldMatch: true,
		},
		{
			name: "matched blob exclude",
			items: []string{
				"!pr*s_",
			},
			input:       "process_",
			shouldMatch: false,
		},
		{
			name: "matched regex exclude",
			items: []string{
				"!/^process_/",
			},
			input:       "process_",
			shouldMatch: false,
		},
		{
			name: "multiples: unmatched single exclude",
			items: []string{
				"other",
				"!app",
			},
			input:       "something",
			shouldMatch: true,
		},
		{
			name: "multiples: matched single exclude",
			items: []string{
				"other",
				"!app",
			},
			input:       "app",
			shouldMatch: false,
		},
		{
			name: "multiples: single match with double regex",
			items: []string{
				"/^process_/",
				"/^node_/",
			},
			input:       "process_",
			shouldMatch: true,
		},
		{
			name: "multiples: double unmatched regex exclude",
			items: []string{
				"!app",
				"!/^process_/",
			},
			input:       "other",
			shouldMatch: true,
		},
		{
			name: "multiples: single match with double regex exclude",
			items: []string{
				"!other",
				"!/^process_/",
			},
			input:       "other",
			shouldMatch: false,
		},
		{
			name: "multiples: unmatched single regex exclude",
			items: []string{
				"app",
				"!/^process_/",
			},
			input:       "other",
			shouldMatch: true,
		},
		{
			name: "multiples: unmatched with single regex",
			items: []string{
				"asdfdfasdf",
				"/^node_/",
			},
			input:       "process_",
			shouldMatch: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := newStringMatcher(tc.items)
			assert.Nil(t, err)
			assert.Equal(t, tc.shouldMatch, f.matches(tc.input))
		})
	}
}

func TestIsNegatedItem(t *testing.T) {
	item, isNegated := isNegatedItem("notNegated")
	assert.Equal(t, "notNegated", item)
	assert.False(t, isNegated)

	item, isNegated = isNegatedItem("!isNegated")
	assert.Equal(t, "isNegated", item)
	assert.True(t, isNegated)
}

func TestIsGlobbed(t *testing.T) {
	assert.True(t, isGlobbed("*"))
	assert.True(t, isGlobbed("a[b-z]"))
	assert.True(t, isGlobbed("{one,two,three}"))
	assert.True(t, isGlobbed("?ingle"))
	assert.True(t, isGlobbed("[!a]"))
	assert.False(t, isGlobbed("notGlobbed"))
}

func TestInvalidStringMatchers(t *testing.T) {
	for _, tc := range []struct {
		name          string
		filter        []string
		expectedError string
	}{
		{
			name: "invalid regex",
			filter: []string{
				"!/\\Z/", // Perl's \Z isn't supported
			},
			expectedError: "invalid regex item: error parsing regexp: invalid escape sequence: `\\Z`",
		},
		{
			name: "invalid glob",
			filter: []string{
				"[",
			},
			expectedError: "invalid glob item: unexpected end of input",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := newStringMatcher(tc.filter)
			assert.Nil(t, f)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError, err.Error())
		})
	}
}
