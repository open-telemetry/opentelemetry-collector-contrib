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
	parent   *Trie
}

// NewTrie allocates and returns a new *Trie.
func NewTrie() *Trie {
	return &Trie{}
}

func (trie *Trie) HasKey(key []byte) bool {
	node := trie
	isEnd := false
	for _, r := range key {
		node = node.children[r]
		if node == nil {
			return isEnd
		}
		// We have reached end of the current path and all the previous characters have matched
		// Return if current node is leaf and it is not root
		if node.isLeaf() && node != trie {
			return true
		}
		// check for any ending node in our current path
		isEnd = isEnd || node.isEnd
	}
	return isEnd
}

// Put inserts the key into the trie
func (trie *Trie) Put(key []byte) {
	node := trie
	shouldPush := false
	var last *Trie
	for _, r := range key {
		child, ok := node.children[r]
		if !ok {
			if node.children == nil {
				node.children = map[byte]*Trie{}
			}
			child = NewTrie()
			child.parent = node
			node.children[r] = child
		}
		node = child
		if node.isEnd {
			last = node
			shouldPush = true
		}
	}
	if shouldPush {
		last.isEnd = false
	}
	node.isEnd = true
}

// Delete removes keys from the Trie. Returns true if node was found for the given key.
// If the node or any of its ancestors
// becomes childless as a result, it is removed from the trie.
func (trie *Trie) Delete(key []byte) bool {
	node := trie
	for _, b := range key {
		node = node.children[b]
		if node == nil {
			// node does not exist
			return false
		}
	}
	return trie.DeleteNode(node)
}

func (trie *Trie) DeleteNode(node *Trie) bool {
	if !node.isEnd {
		return false
	}
	node.isEnd = false
	// if leaf, remove it from its parent's children map.
	if node.isLeaf() {
		// iterate backwards over path, until we reach root
		for node != trie {
			parent := node.parent
			for key, child := range parent.children {
				if node == child {
					// delete the key from parent's map
					delete(parent.children, key)
					break
				}
			}

			if !parent.isLeaf() {
				// parent has other children, stop
				break
			}
			parent.children = nil
			if parent.isEnd {
				// Parent has a value, stop
				break
			}
			node = node.parent
		}
	}
	return true // node (internal or not) existed and its value was nil'd
}

func (trie *Trie) isLeaf() bool {
	return len(trie.children) == 0
}
