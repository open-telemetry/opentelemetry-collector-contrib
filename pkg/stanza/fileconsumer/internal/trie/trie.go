// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// TRIE data structure inspired by https://github.com/dghubble/trie
/*
	This differs from the original trie.
   	This has been modified to detect partial matches as well.
   	For eg.
		If we add "ABCD" to this trie, and try to check if "ABCDEF" is in the trie,
		it will return true because that's how fingerprint matching works in current implementation.
*/

package trie // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/trie"

type Trie[T any] struct {
	value    *T
	children map[byte]*Trie[T]
}

// NewTrie allocates and returns a new *Trie.
func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{children: make(map[byte]*Trie[T])}
}

func (trie *Trie[T]) Get(key []byte) T {
	node := trie
	var value T
	for _, b := range key {
		node = node.children[b]
		if node == nil {
			return value // end of trie
		}
		if node.value != nil {
			value = *node.value
		}
	}
	return value // end of fingerprint
}

// TODO []byte, T value parameters
// Put inserts a value into the trie
func (trie *Trie[T]) Put(key []byte, v T) {
	node := trie
	for _, b := range key {
		_, ok := node.children[b]
		if !ok {
			node.children[b] = NewTrie[T]()
		}
		node = node.children[b]
	}
	node.value = &v
}

// Delete removes a value from the Trie. Returns true if the value was found.
// Any empty nodes which become childless as a result are removed from the trie.
func (trie *Trie[T]) Delete(key []byte) bool {
	var path []*Trie[T] // record ancestors to check later
	node := trie
	for _, b := range key {
		node = node.children[b]
		if node == nil {
			return false
		}
		path = append(path, node)
	}

	if node.value == nil {
		return false
	}

	// Remove the value
	node.value = nil

	// Iterate back up the path and remove empty nodes
	for i := len(path) - 1; i >= 0; i-- {
		node := path[i]
		b := key[i]
		delete(node.children, b)
		if len(node.children) > 0 || node.value != nil {
			break // node has other children or a value, leave it
		}
	}

	return true
}
