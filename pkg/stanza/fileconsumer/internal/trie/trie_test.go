// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trie // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/trie"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrie(t *testing.T) {
	// Run all the test cases in sequential order on the same trie to test the expected behavior
	trie := NewTrie()
	testCases := []struct {
		value          []byte
		matchExpected  bool
		delete         bool // If we should delete the given value from the trie
		deleteExpected bool
	}{
		{
			value:          []byte("ABCD"),
			matchExpected:  false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCDEFG"),
			matchExpected:  true,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCEFG"),
			matchExpected:  false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCD"),
			matchExpected:  false,
			delete:         true,
			deleteExpected: true,
		},
		{
			value:          []byte("ABCDEFG"),
			matchExpected:  false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("XYZ"),
			matchExpected:  false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCEFGH"),
			matchExpected:  true,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:         []byte("QWERTY"),
			matchExpected: false,
			delete:        true,
			// should be false as we haven't added any such value in Trie
			deleteExpected: false,
		},
		{
			value:          []byte("XYZ"),
			matchExpected:  true,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCDEFG"),
			matchExpected:  true,
			delete:         false,
			deleteExpected: false,
		},
	}
	for _, tc := range testCases {
		if tc.delete {
			// Delete the value and check if it was deleted successfully
			assert.Equal(t, trie.Delete(tc.value), tc.deleteExpected)
		} else {
			assert.Equal(t, trie.HasKey(tc.value), tc.matchExpected)
			if !tc.matchExpected {
				assert.Equal(t, trie.HasKey(tc.value), false)
				trie.Put(tc.value)
			}
		}
	}
}
