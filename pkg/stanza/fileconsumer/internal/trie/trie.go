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

type Trie struct {
	isEnd    bool
	children map[byte]*Trie
}

// NewTrie allocates and returns a new *Trie.
func NewTrie() *Trie {
	return &Trie{}
}

func (trie *Trie) HasKey(key []byte) bool {
	node := trie
	for _, r := range key {
		node = node.children[r]
		if node == nil {
			return false
		}
		// We have reached end of the current path and all the previous characters have matched
		// Return if current node is leaf and it is not root
		if node.isLeaf() && node != trie {
			return true
		}
	}
	return false //  If it's an exact match, the final node will be a leaf.
}

// Put inserts the key into the trie
func (trie *Trie) Put(key []byte) {
	node := trie
	for _, r := range key {
		child, ok := node.children[r]
		if !ok {
			if node.children == nil {
				node.children = map[byte]*Trie{}
			}
			child = NewTrie()
			node.children[r] = child
		}
		node = child
	}
	node.isEnd = true
}

// Delete removes keys from the Trie. Returns true if node was found for the given key.
// If the node or any of its ancestors
// becomes childless as a result, it is removed from the trie.
func (trie *Trie) Delete(key []byte) bool {
	var path []*Trie // record ancestors to check later
	node := trie
	for _, b := range key {
		path = append(path, node)
		node = node.children[b]
		if node == nil {
			// node does not exist
			return false
		}
	}
	node.isEnd = false
	// if leaf, remove it from its parent's children map. Repeat for ancestor path.
	if node.isLeaf() {
		// iterate backwards over path
		for i := len(path) - 1; i >= 0; i-- {
			parent := path[i]
			b := key[i]
			delete(parent.children, b)
			if !parent.isLeaf() {
				// parent has other children, stop
				break
			}
			parent.children = nil
			if parent.isEnd {
				// Parent has a value, stop
				break
			}
		}
	}
	return true // node (internal or not) existed and its value was nil'd
}

func (trie *Trie) isLeaf() bool {
	return len(trie.children) == 0
}
