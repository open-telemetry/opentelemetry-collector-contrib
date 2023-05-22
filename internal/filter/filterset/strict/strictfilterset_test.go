// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package strict

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	validStrictFilters = []string{
		"exact_string_match",
		".*/suffix",
		"(a|b)",
	}
)

func TestNewStrictFilterSet(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		success bool
	}{
		{
			name:    "validFilters",
			filters: validStrictFilters,
			success: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs := NewFilterSet(test.filters)
			assert.Equal(t, test.success, fs != nil)
		})
	}
}

func TestStrictMatches(t *testing.T) {
	fs := NewFilterSet(validStrictFilters)
	assert.NotNil(t, fs)

	matches := []string{
		"exact_string_match",
		".*/suffix",
		"(a|b)",
	}

	for _, m := range matches {
		t.Run(m, func(t *testing.T) {
			assert.True(t, fs.Matches(m))
		})
	}

	mismatches := []string{
		"not_exact_string_match",
		"random",
		"test/match/suffix",
		"prefix/metric/one",
		"c",
	}

	for _, m := range mismatches {
		t.Run(m, func(t *testing.T) {
			assert.False(t, fs.Matches(m))
		})
	}
}
