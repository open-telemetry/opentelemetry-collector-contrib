// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// TRIE data structure inspired by https://github.com/dghubble/trie
/*
	This differs from the original trie.
   	This has been modified to detect partial matches as well.
   	For eg.
		If we add "ABCDEF" to this trie, and try to check if "ABCD" is in the trie,
		it will return true because that's how fingerprint matching works in current implementation.
*/

package fileconsumer

type Trie struct {
	value    interface{}
	children map[byte]*Trie
}

// Trie node and the part string key of the child the path descends into.
type nodeTrie struct {
	node *Trie
	b    byte
}

// NewPathTrie allocates and returns a new *Trie.
func NewTrie() *Trie {
	return &Trie{}
}

func (trie *Trie) Get(key []byte) interface{} {
	node := trie
	for _, r := range key {
		node = node.children[r]
		if node == nil {
			return nil
		}
		// We have reached end of the current path and all the previous characters have matched
		// Return if current node is leaf and it is not root
		if node.isLeaf() && node != trie {
			return node.value
		}
	}
	return node.value
}

// Put inserts the value into the trie at the given key
func (trie *Trie) Put(key []byte, value interface{}) {
	node := trie
	for _, r := range key {
		child, _ := node.children[r]
		if child == nil {
			if node.children == nil {
				node.children = map[byte]*Trie{}
			}
			child = NewTrie()
			node.children[r] = child

			// Assiging value to every child node allows us to detect partial matches.
			// For eg. `123451` and `123456789` will match, even if they are not exactly same strings.
			// Doing this, we store every prefix of the fingerprint.
			node.value = value
		}
		node = child
	}
	node.value = value
}

// Delete removes the value associated with the given key. Returns true if a
// node was found for the given key. If the node or any of its ancestors
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
	// delete the node value
	node.value = nil
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
			parent.value = nil
		}
	}
	return true // node (internal or not) existed and its value was nil'd
}

func (trie *Trie) isLeaf() bool {
	return len(trie.children) == 0
}
