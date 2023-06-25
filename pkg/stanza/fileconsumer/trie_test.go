// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTrie(t *testing.T) {
	// Run all the test cases in sequential order on the same trie to test the expected behaviour
	trie := NewTrie()
	testCases := []struct {
		value          []byte
		matchFound     bool
		delete         bool
		deleteExpected bool
	}{
		{
			value:          []byte("ABCDEFG"),
			matchFound:     false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCD"),
			matchFound:     true,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCEFG"),
			matchFound:     false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCDEFG"),
			matchFound:     false,
			delete:         true,
			deleteExpected: true,
		},
		{
			value:          []byte("ABCD"),
			matchFound:     false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("XYZ"),
			matchFound:     false,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:          []byte("ABCEF"),
			matchFound:     true,
			delete:         false,
			deleteExpected: false,
		},
		{
			value:      []byte("QWERTY"),
			matchFound: false,
			delete:     true,
			// should be false as we haven't added any such value in Trie
			deleteExpected: false,
		},
	}
	for _, tc := range testCases {

		if tc.delete {
			// Delete the value and check if it was deleted successfully
			assert.Equal(t, trie.Delete(tc.value), tc.deleteExpected)
		} else {
			if tc.matchFound {
				assert.NotNil(t, trie.Get(tc.value))
			} else {
				assert.Nil(t, trie.Get(tc.value))
				trie.Put(tc.value, true)
			}
		}
	}
}
