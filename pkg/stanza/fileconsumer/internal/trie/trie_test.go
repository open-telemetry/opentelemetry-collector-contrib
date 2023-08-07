// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trie // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/trie"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	value          []byte
	matchExpected  bool
	delete         bool // If we should delete the given value from the trie
	deleteExpected bool
}

type trieTest struct {
	initialItems []string
	testCases    []testCase
	name         string
}

func TestTrie(t *testing.T) {
	// Run all the test cases in sequential order on the same trie to test the expected behavior
	testCases := []trieTest{
		{
			name:         "TrieCase_Normal",
			initialItems: []string{"ABCD", "XYZ"},
			testCases: []testCase{
				{
					value:         []byte("ABCDEFG"),
					matchExpected: true,
				},
				{
					value:          []byte("ABCD"),
					matchExpected:  false,
					delete:         true,
					deleteExpected: true,
				},
				{
					value:         []byte("ABCDEFG"),
					matchExpected: false,
				},
				{
					value:         []byte("XYZ"),
					matchExpected: true,
				},
				{
					value:         []byte("XYZBlaBla"),
					matchExpected: true,
				},
				{
					value:         []byte("X"),
					matchExpected: false,
				},
			},
		},
		{
			name:         "TrieCase_SimilarKeys_1",
			initialItems: []string{"ABCDEFG", "ABCD"},
			testCases: []testCase{
				{
					value:          []byte("ABCDEFG"),
					matchExpected:  false,
					delete:         true,
					deleteExpected: true,
				},
				{
					value:         []byte("ABCD"),
					matchExpected: true,
				},
				{
					value:         []byte("ABCDEFG"),
					matchExpected: true,
				},
				{
					value:         []byte("ABCDEFGHI"),
					matchExpected: true,
				},
			},
		},
		{
			name:         "TrieCase_SimilarKeys_2",
			initialItems: []string{"ABCDEFG", "ABCD"},
			testCases: []testCase{
				{
					value:          []byte("ABCD"),
					delete:         true,
					deleteExpected: true,
				},
				{
					value:         []byte("ABCD"),
					matchExpected: false,
				},
				{
					value:         []byte("ABCDEF"),
					matchExpected: true,
				},
				{
					value:         []byte("ABCDEFGHI"),
					matchExpected: true,
				},
			},
		},
		{
			name:         "TrieCase_Different",
			initialItems: []string{"ABCD", "XYZ"},
			testCases: []testCase{
				{
					value: []byte("ABCEFG"),
				},
				{
					value:         []byte("ABCDXYZ"),
					matchExpected: true,
				},
				{
					value: []byte("ABXE"),
				},
			},
		},
		{
			name:         "TrieCase_Exact",
			initialItems: []string{"ABCDEFG", "ABCD"},
			testCases: []testCase{
				{
					value:         []byte("ABCDEFG"),
					matchExpected: true,
				},
				{
					value:         []byte("ABCD"),
					matchExpected: true,
				},
				{
					value:         []byte("ABCDE"),
					matchExpected: true,
				},
			},
		},
		{
			name:         "TrieCase_DeleteFalse",
			initialItems: []string{"ABCDEFG"},
			testCases: []testCase{
				{
					value:          []byte("ABCDEFG"),
					delete:         true,
					deleteExpected: true,
				},
				{
					value: []byte("ABCD"),
				},
				{
					value:  []byte("XYZ"),
					delete: true,
					// it should be false, as we haven't inserted such values
					deleteExpected: false,
				},
			},
		},
		{
			name:         "TrieCase_Complex",
			initialItems: []string{"ABCDE", "ABC"},
			testCases: []testCase{
				{
					value:         []byte("ABCDEXYZ"),
					matchExpected: true,
				},
				{
					value:         []byte("ABCXYZ"),
					matchExpected: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		trie := NewTrie()
		for _, k := range tc.initialItems {
			trie.Put([]byte(k))
		}
		t.Run(tc.name, func(t *testing.T) {
			for _, T := range tc.testCases {
				runTest(t, trie, &T)
			}
		})
	}
}

func runTest(t *testing.T, trie *Trie, tc *testCase) {
	if tc.delete {
		// Delete the value and check if it was deleted successfully
		assert.Equal(t, trie.Delete(tc.value), tc.deleteExpected)
	} else {
		assert.Equal(t, trie.HasKey(tc.value), tc.matchExpected)
		if !tc.matchExpected {
			trie.Put(tc.value)
		}
	}
}

func TestTrieOpSequences(t *testing.T) {
	opTree{
		continuations: map[string]opTree{
			"Found:ABC": opTree{ // First poll finds only ABC.
				ops: []testOp{
					put("ABC"),
				},
				continuations: map[string]opTree{
					"Done:ABC": opTree{ // Finish reading ABC and remove from trie
						ops: []testOp{
							del("ABC", true, "was just added"),
							has("ABC", false, "was just deleted"),
						},
					},
					"Found:ABCDEF": opTree{ // Next poll finds ABCDEF
						ops: []testOp{
							has("ABCDEF", true, "recognize ABC w/ DEF appended"),
							put("ABCDEF"),
							has("ABC", true, "should not have effect on ABCDEF"),
						},

						continuations: map[string]opTree{
							"DeleteAs:ABC": opTree{ // Done reading the file, remove it as ABC
								ops: []testOp{
									del("ABC", true, "should be deleted"),
									has("ABCDEF", true, "should not have been deleted"),
								},
								continuations: map[string]opTree{
									"DeleteAs:ABCDEF": opTree{ // Also remove it as ABCDEF
										ops: []testOp{
											del("ABCDEF", true, "just confirmed it exists"), // TODO this fails to delete
										},
									},
								},
							},
							"DeleteAs:ABCDEF": opTree{ // Done reading the file, remove it as ABC
								ops: []testOp{
									del("ABCDEF", true, "trying to delete ABC should not affect ABCDEF"),
									has("ABC", true, "should not have been deleted"),
								},
								continuations: map[string]opTree{
									"DeleteAs:ABC": opTree{ // Also remove it as ABC
										ops: []testOp{
											del("ABC", true, "just confirmed it exists"), // TODO this fails to delete
										},
									},
								},
							},
						},
					},
				},
			},
			"Found:ABC,ABCDEF": opTree{ // First poll finds ABCDEF and ABC.
				ops: []testOp{
					// In order to avoid overwriting ABC with ABCDEF, we need to add ABCDEF first.
					// TODO Should poll results be sorted by decreasing length before adding to trie?
					put("ABCDEF"),
					put("ABC"),
					has("ABCDEF", true, "adding ABC after ABCDEF shouldn't affect ABCDEF"),
				},
				continuations: map[string]opTree{
					"Done:ABC": opTree{ // Finish reading ABC and remove from trie
						ops: []testOp{
							del("ABC", true, "just confirmed ABC exists"),
							has("ABCDEF", true, "ABCDEF should not have been deleted"),
						},
						continuations: map[string]opTree{
							"Done:ABCDEF": opTree{ // Finish reading ABCDEF and remove from trie
								ops: []testOp{
									del("ABCDEF", true, "just confirmed ABCDEF exists"),
								},
							},
						},
					},
					"Done:ABCDEF": opTree{ // Finish reading ABCDEF and remove from trie
						ops: []testOp{
							del("ABCDEF", true, "just confirmed ABCDEF exists"),
							has("ABC", true, "should not have been deleted"),
						},
						continuations: map[string]opTree{
							"Done:ABC": opTree{ // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABC", true, "just confirmed ABC exists"),
								},
							},
						},
					},
					"Found:ABCxyz,ABCDEF": opTree{ // Next poll finds ABCxyz and ABCDEF
						ops: []testOp{
							has("ABCxyz", true, "recognize ABC w/ xyz appended"),
							put("ABCxyz"), // TODO how do we know we need to call this?
							has("ABC", true, "ABCxyz should not have effect on ABC"),
							has("ABCDEF", true, "ABCDEF should not have been affected"),
						},
						continuations: map[string]opTree{
							"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
								ops: []testOp{
									del("ABCxyz", true, "just confirmed ABCxyz exists"),
									has("ABC", true, "should be present"),
									has("ABCDEF", true, "ABCDEF should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEF": opTree{ // Finish reading ABCDEF and remove from trie
										ops: []testOp{
											del("ABCDEF", true, "just confirmed ABCDEF exists"),
										},
									},
									"Found:ABCDEFxyz": opTree{ // Next poll finds ABCDEFxyz
										ops: []testOp{
											has("ABCDEFxyz", true, "recognize ABCDEF w/ xyz appended"),
											put("ABCDEFxyz"),
										},
										continuations: map[string]opTree{
											"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
												ops: []testOp{
													del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
													has("ABCDEF", true, "ABCDEF should not have been deleted"),
												},
											},
										},
									},
								},
							},
							"Done:ABC": opTree{ // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABC", true, "just confirmed ABCxyz exists"),
									has("ABCxyz", true, "ABCxyz should be present"),
									has("ABCDEF", true, "ABCDEF should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEF": opTree{ // Finish reading ABCDEF and remove from trie
										ops: []testOp{
											del("ABCDEF", true, "just confirmed ABCDEF exists"),
										},
									},
									"Found:ABCDEFxyz": opTree{ // Next poll finds ABCDEFxyz
										ops: []testOp{
											has("ABCDEFxyz", true, "recognize ABCDEF w/ xyz appended"),
											put("ABCDEFxyz"), // TODO how do we know we need to call this?
										},
										continuations: map[string]opTree{
											"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
												ops: []testOp{
													del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
												},
											},
										},
									},
								},
							},
							"Done:ABCDEF": opTree{ // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABCDEF", true, "just confirmed ABCDEF exists"),
									has("ABCxyz", true, "should be present"),
									has("ABC", true, "ABC should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABC": opTree{ // Finish reading ABCDEF and remove from trie
										ops: []testOp{
											del("ABC", true, "just confirmed ABCDEF exists"),
											has("ABCxyz", true, "ABCxyz should be present"),
										},
									},
									"Done:ABCxyz": opTree{
										ops: []testOp{
											del("ABCxyz", true, "just confirmed ABCxyz exists"),
											has("ABC", true, "ABC should be present"),
										},
									},
									"Found:ABCDEFxyz": opTree{ // Next poll finds ABCDEFxyz
										ops: []testOp{
											has("ABCDEFxyz", true, "recognize ABCDEF w/ xyz appended"),
											put("ABCDEFxyz"), // TODO how do we know we need to call this?
										},
										continuations: map[string]opTree{
											"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
												ops: []testOp{
													del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
													has("ABCDEF", true, "ABCDEF should be present"),
												},
											},
										},
									},
								},
							},
							"Found:ABCxyz,ABCDEFxyz": opTree{ // Next poll finds ABCxyz and ABCDEFxyz
								ops: []testOp{
									has("ABCDEFxyz", true, "recognize ABCDEF w/ xyz appended"),
									put("ABCDEFxyz"), // TODO how do we know we need to call this?
									has("ABCxyz", true, "ABCxyz should not have been affected"),
								},
								continuations: map[string]opTree{
									"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
										ops: []testOp{
											del("ABCxyz", true, "just confirmed ABCxyz exists"),
											has("ABCDEFxyz", true, "ABCDEFxyz should not have been deleted"),
										},
										continuations: map[string]opTree{
											"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
												ops: []testOp{
													del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
												},
											},
										},
									},
									"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
										ops: []testOp{
											del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
											has("ABCxyz", true, "ABCxyz should not have been deleted"),
										},
										continuations: map[string]opTree{
											"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
												ops: []testOp{
													del("ABCxyz", true, "just confirmed ABCxyz exists"),
												},
											},
										},
									},
								},
							},
						},
					},
					"Found:ABC,ABCDEFxyz": opTree{ // Next poll finds ABC and ABCDEFxyz
						ops: []testOp{
							has("ABCDEFxyz", true, "recognize ABCDEF w/ xyz appended"),
							put("ABCDEFxyz"), // TODO how do we know we need to call this?
							has("ABCDEF", true, "ABCDEF should be present"),
							has("ABC", true, "ABC should not have been affected"),
						},
						continuations: map[string]opTree{
							"Done:ABC": opTree{ // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABC", true, "just confirmed ABC exists"),
									has("ABCDEFxyz", true, "ABCDEFxyz should not have been deleted"),
									has("ABCDEF", true, "ABCDEF should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
										ops: []testOp{
											del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
											has("ABCDEF", true, "ABCDEF should not have been deleted"),
										},
									},
								},
							},
							"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
								ops: []testOp{
									del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
									has("ABC", true, "ABC should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABC": opTree{ // Finish reading ABC and remove from trie
										ops: []testOp{
											del("ABC", true, "just confirmed ABC exists"),
										},
									},
									"Found:ABCxyz": opTree{ // Next poll finds ABCxyz
										ops: []testOp{
											has("ABCxyz", true, "recognize ABC w/ xyz appended"),
											put("ABCxyz"), // TODO how do we know we need to call this?
										},
										continuations: map[string]opTree{
											"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
												ops: []testOp{
													del("ABCxyz", true, "just confirmed ABCxyz exists"),
												},
											},
										},
									},
								},
							},
							"Found:ABCxyz,ABCDEFxyz": opTree{ // Next poll finds ABCxyz and ABCDEFxyz
								ops: []testOp{
									has("ABCxyz", true, "recognize ABC w/ xyz appended"),
									put("ABCxyz"), // TODO how do we know we need to call this?
									has("ABCDEFxyz", true, "ABCDEFxyz should not have been affected"),
								},
								continuations: map[string]opTree{
									"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
										ops: []testOp{
											del("ABCxyz", true, "just confirmed ABCxyz exists"),
											has("ABCDEFxyz", true, "ABCDEFxyz should not have been deleted"),
										},
										continuations: map[string]opTree{
											"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
												ops: []testOp{
													del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
												},
											},
										},
									},
									"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
										ops: []testOp{
											del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
											has("ABCxyz", true, "ABCxyz should not have been deleted"),
										},
										continuations: map[string]opTree{
											"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
												ops: []testOp{
													del("ABCxyz", true, "just confirmed ABCxyz exists"),
												},
											},
										},
									},
								},
							},
						},
					},
					"Found:ABCxyz,ABCDEFxyz": opTree{ // Next poll finds ABCxyz and ABCDEFxyz
						ops: []testOp{
							// Inserting longer string first
							has("ABCDEFxyz", true, "recognize ABCDEF w/ xyz appended"),
							put("ABCDEFxyz"),
							has("ABCDEF", true, "ABCDEF should be present"),
							has("ABC", true, "ABC should not have been affected"),

							has("ABCxyz", true, "recognize ABC w/ xyz appended"),
							put("ABCxyz"),
							has("ABC", true, "ABC should be present"),
						},
						continuations: map[string]opTree{
							"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
								ops: []testOp{
									del("ABCxyz", true, "just confirmed ABCxyz exists"),
									has("ABCDEFxyz", true, "ABCDEFxyz should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
										ops: []testOp{
											del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
										},
									},
								},
							},
							"Done:ABCDEFxyz": opTree{ // Finish reading ABCDEFxyz and remove from trie
								ops: []testOp{
									del("ABCDEFxyz", true, "just confirmed ABCDEFxyz exists"),
									has("ABCxyz", true, "ABCxyz should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABCxyz": opTree{ // Finish reading ABCxyz and remove from trie
										ops: []testOp{
											del("ABCxyz", true, "just confirmed ABCxyz exists"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}.run(t, []testOp{})(t)
}

// testOp is one HasKey, Put, or Delete call to the trie,
// along with validation of expectations.
type testOp func(t *testing.T, trie *Trie)

func has(key string, expect bool, why string) testOp {
	return func(t *testing.T, trie *Trie) {
		assert.Equalf(t, trie.HasKey([]byte(key)), expect, why)
	}
}

// put automatically asserts that the trie contains the key after adding.
func put(key string) testOp {
	return func(t *testing.T, trie *Trie) {
		trie.Put([]byte(key))
		assert.Truef(t, trie.HasKey([]byte(key)), "called Put(%s) but HasKey(%s) is still false", key, key)
	}
}

// del automatically asserts that the trie no longer contains the key after deleting it.
func del(key string, expect bool, why string) testOp {
	return func(t *testing.T, trie *Trie) {
		assert.Equalf(t, trie.Delete([]byte(key)), expect, why)
	}
}

// opTree represents many possible sequences of operations that may be performed on a trie.
// Each opTree represents a stage at which a concrete sequence of operations should occur "now".
// An opTree's "continuations" are possible "futures" that may occur next, each of which may have a variety of further continuations.
// The tree structure allows us to thoroughly explore the space of possible sequences without having to define the same setup steps over and over.
type opTree struct {
	ops           []testOp
	continuations map[string]opTree
}

func (ot opTree) run(t *testing.T, opSequence []testOp) func(*testing.T) {
	return func(t *testing.T) {
		trie := NewTrie()
		opSequence = append(opSequence, ot.ops...)
		for _, op := range opSequence {
			op(t, trie)
		}
		if t.Failed() {
			// All continuations will fail at the same point, so don't bother running them
			return
		}
		for name, continuation := range ot.continuations {
			t.Run(name, continuation.run(t, opSequence))
		}
	}
}
