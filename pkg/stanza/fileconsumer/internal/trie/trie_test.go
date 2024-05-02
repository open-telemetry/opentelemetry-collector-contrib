// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trie // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/trie"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrie(t *testing.T) {
	opTree{
		continuations: map[string]opTree{
			"Found:ABC": { // First poll finds only ABC.
				ops: []testOp{
					get("ABC", nil, "empty trie"),
					put("ABC", 3),
				},
				continuations: map[string]opTree{
					"Done:ABC": { // Finish reading ABC and remove from trie
						ops: []testOp{
							del("ABC", "was just added"),
							get("ABC", nil, "was just deleted"),
						},
					},
					"Found:ABCDEF": { // Next poll finds ABCDEF
						ops: []testOp{
							get("ABCDEF", 3, "recognize ABC w/ DEF appended"),
						},

						continuations: map[string]opTree{
							"Done:ABC": { // Done reading the file, remove it as ABC
								ops: []testOp{
									del("ABC", "should be deleted"),
									get("ABC", nil, "should have been deleted"),
								},
							},
						},
					},
				},
			},
			"Found:ABC,ABCDEF": { // First poll finds ABCDEF and ABC.
				ops: []testOp{
					// In order to avoid overwriting ABC with ABCDEF, we need to add ABCDEF first.
					// TODO Should poll results be sorted by decreasing length before adding to trie?
					get("ABCDEF", nil, "empty trie"),
					put("ABCDEF", 6),
					get("ABC", nil, "ABC should not be in trie yet"),
					put("ABC", 3),
					get("ABCDEF", 6, "make sure adding ABC did not change value of ABCDEF"),
				},
				continuations: map[string]opTree{
					"Done:ABC": { // Finish reading ABC and remove from trie
						ops: []testOp{
							del("ABC", "just confirmed ABC exists"),
							get("ABC", nil, "ABC should have been deleted"),
							get("ABCDEF", 6, "ABCDEF should not have been deleted"),
						},
						continuations: map[string]opTree{
							"Done:ABCDEF": { // Finish reading ABCDEF and remove from trie
								ops: []testOp{
									del("ABCDEF", "just confirmed ABCDEF exists"),
									get("ABCDEF", nil, "ABCDEF should have been deleted"),
								},
							},
						},
					},
					"Done:ABCDEF": { // Finish reading ABCDEF and remove from trie
						ops: []testOp{
							del("ABCDEF", "just confirmed ABCDEF exists"),
							get("ABC", 3, "should not have been deleted"),
							// get(ABCDEF) will still return true because ABC is still in trie
						},
						continuations: map[string]opTree{
							"Done:ABC": { // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABC", "just confirmed ABC exists"),
									get("ABC", nil, "just deleted ABC"),
								},
							},
						},
					},
					"Found:ABCxyz,ABCDEF": { // Next poll finds ABCxyz and ABCDEF
						ops: []testOp{
							get("ABCxyz", 3, "recognize ABC w/ xyz appended"),
							get("ABCDEF", 6, "recognize ABCDEF"),
						},
						continuations: map[string]opTree{
							"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
								ops: []testOp{
									del("ABC", "should still be known as ABC"),
									get("ABC", nil, "just deleted ABC"),
									get("ABCDEF", 6, "ABCDEF should not have been affected"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEF": { // Finish reading ABCDEF and remove from trie
										ops: []testOp{
											del("ABCDEF", "just confirmed ABCDEF exists"),
											get("ABCDEF", nil, "ABCDEF should have been deleted"),
										},
									},
									"Found:ABCDEFxyz": { // Next poll finds ABCDEFxyz
										ops: []testOp{
											get("ABCDEFxyz", 6, "recognize ABCDEF w/ xyz appended"),
										},
										continuations: map[string]opTree{
											"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
												ops: []testOp{
													del("ABCDEF", "just confirmed ABCDEFxyz exists"),
													get("ABCDEF", nil, "ABCDEFxyz should have been deleted"),
												},
											},
										},
									},
								},
							},
							"Done:ABCDEF": { // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABCDEF", "just confirmed ABCDEF exists"),
									get("ABCxyz", 3, "should still exist as ABC"),
								},
								continuations: map[string]opTree{
									"Done:ABC": { // Finish reading ABCDEF and remove from trie
										ops: []testOp{
											del("ABC", "just confirmed ABC exists"),
											get("ABC", nil, "just deleted ABC"),
											get("ABCDEF", nil, "deleted this earlier but get(ABCDEF) was true until after del(ABC)"),
										},
									},
									"Found:ABCxyz": { // Next poll finds ABCDEFxyz
										ops: []testOp{
											get("ABCxyz", 3, "recognize ABC w/ xyz appended"),
										},
										continuations: map[string]opTree{
											"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
												ops: []testOp{
													del("ABC", "still known as ABC"),
													get("ABC", nil, "just deleted ABC"),
													get("ABCDEF", nil, "deleted this earlier but get(ABCDEF) was true until after del(ABC)"),
												},
											},
										},
									},
								},
							},
							"Found:ABCxyz,ABCDEFxyz": { // Next poll finds ABCxyz and ABCDEFxyz
								ops: []testOp{
									get("ABCxyz", 3, "recognize ABC w/ xyz appended"),
									get("ABCDEFxyz", 6, "recognize ABCDEF w/ xyz appended"),
								},
								continuations: map[string]opTree{
									"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
										ops: []testOp{
											del("ABC", "should still be present as ABC"),
											get("ABC", nil, "just deleted ABC"),
											get("ABCDEFxyz", 6, "should still exist as ABCDEF"),
										},
										continuations: map[string]opTree{
											"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
												ops: []testOp{
													del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
													get("ABCDEF", nil, "just deleted ABCDEF"),
												},
											},
										},
									},
									"Done:ABCDEF": { // Finish reading ABCDEFxyz and remove from trie
										ops: []testOp{
											del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
											get("ABCxyz", 3, "should still exist as ABC"),
										},
										continuations: map[string]opTree{
											"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
												ops: []testOp{
													del("ABC", "just confirmed ABCxyz exists"),
													get("ABC", nil, "just deleted ABC"),
												},
											},
										},
									},
								},
							},
						},
					},
					"Found:ABC,ABCDEFxyz": { // Next poll finds ABC and ABCDEFxyz
						ops: []testOp{
							get("ABC", 3, "recognize ABC"),
							get("ABCDEFxyz", 6, "recognize ABCDEF w/ xyz appended"),
						},
						continuations: map[string]opTree{
							"Done:ABC": { // Finish reading ABC and remove from trie
								ops: []testOp{
									del("ABC", "just confirmed ABC exists"),
									get("ABC", nil, "just deleted ABC"),
									get("ABCDEFxyz", 6, "should still exist as ABCDEF"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
										ops: []testOp{
											del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
											get("ABCDEF", nil, "just deleted ABCDEF"),
										},
									},
								},
							},
							"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
								ops: []testOp{
									del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
									get("ABC", 3, "ABC should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABC": { // Finish reading ABC and remove from trie
										ops: []testOp{
											del("ABC", "just confirmed ABC exists"),
											get("ABC", nil, "just deleted ABC"),
										},
									},
									"Found:ABCxyz": { // Next poll finds ABCxyz
										ops: []testOp{
											get("ABCxyz", 3, "recognize ABC w/ xyz appended"),
										},
										continuations: map[string]opTree{
											"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
												ops: []testOp{
													del("ABC", "just confirmed ABC(xyz) exists"),
													get("ABC", nil, "just deleted ABC"),
													get("ABCDEF", nil, "deleted this earlier but get(ABCDEF) was true until after del(ABC)"),
												},
											},
										},
									},
								},
							},
							"Found:ABCxyz,ABCDEFxyz": { // Next poll finds ABCxyz and ABCDEFxyz
								ops: []testOp{
									get("ABCxyz", 3, "recognize ABC w/ xyz appended"),
									get("ABCDEFxyz", 6, "recognize ABCDEF w/ xyz appended"),
								},
								continuations: map[string]opTree{
									"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
										ops: []testOp{
											del("ABC", "just confirmed ABC(xyz) exists"),
											get("ABC", nil, "just deleted ABC"),
											get("ABCDEFxyz", 6, "ABCDEF(xyz) should not have been affected"),
										},
										continuations: map[string]opTree{
											"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
												ops: []testOp{
													del("ABCDEF", "just confirmed ABCDEFxyz exists"),
													get("ABCDEF", nil, "just deleted ABCDEF"),
												},
											},
										},
									},
									"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
										ops: []testOp{
											del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
											get("ABCxyz", 3, "should still exist as ABC"),
										},
										continuations: map[string]opTree{
											"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
												ops: []testOp{
													del("ABC", "just confirmed ABCxyz exists"),
													get("ABC", nil, "just deleted ABC"),
													get("ABCDEF", nil, "deleted this earlier but get(ABCDEF) was true until after del(ABC)"),
												},
											},
										},
									},
								},
							},
						},
					},
					"Found:ABCxyz,ABCDEFxyz": { // Next poll finds ABCxyz and ABCDEFxyz
						ops: []testOp{
							// Process longer string first
							get("ABCDEFxyz", 6, "recognize ABCDEF w/ xyz appended"),
							get("ABCxyz", 3, "recognize ABC w/ xyz appended"),
						},
						continuations: map[string]opTree{
							"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
								ops: []testOp{
									del("ABC", "just confirmed ABC(xyz) exists"),
									get("ABC", nil, "just deleted ABC"),
									get("ABCDEFxyz", 6, "ABCDEF(xyz) should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
										ops: []testOp{
											del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
											get("ABCDEF", nil, "just deleted ABCDEF"),
										},
									},
								},
							},
							"Done:ABCDEF": { // Finish reading ABCDEF(xyz) and remove from trie
								ops: []testOp{
									del("ABCDEF", "just confirmed ABCDEF(xyz) exists"),
									get("ABCxyz", 3, "ABC(xyz) should not have been deleted"),
								},
								continuations: map[string]opTree{
									"Done:ABC": { // Finish reading ABC(xyz) and remove from trie
										ops: []testOp{
											del("ABC", "just confirmed ABC(xyz) exists"),
											get("ABC", nil, "just deleted ABC"),
											get("ABCDEF", nil, "deleted this earlier but get(ABCDEF) was true until after del(ABC)"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}.run([]testOp{})(t)
}

// testOp is one Get, Put, or Delete call to the trie, along with validation of expectations.
type testOp func(t *testing.T, trie *Trie[any])

func get(key string, expect any, why string) testOp {
	return func(t *testing.T, trie *Trie[any]) {
		assert.Equalf(t, expect, trie.Get([]byte(key)), why)
	}
}

// put automatically asserts that the trie contains the key after adding.
func put(key string, val any) testOp {
	return func(t *testing.T, trie *Trie[any]) {
		trie.Put([]byte(key), val)
		assert.Equalf(t, val, trie.Get([]byte(key)), "called Put(%s, %d) but HasKey(%s) does not return %d", key, key)
	}
}

// del automatically asserts that the trie no longer contains the key after deleting it.
func del(key string, why string) testOp {
	return func(t *testing.T, trie *Trie[any]) {
		val := trie.Get([]byte(key))
		if val == nil {
			assert.Falsef(t, trie.Delete([]byte(key)), why)
		} else {
			assert.Truef(t, trie.Delete([]byte(key)), why)
			assert.Falsef(t, trie.Delete([]byte(key)), "called Del(%s) twice in a row and got true both times")
		}
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

func (ot opTree) run(opSequence []testOp) func(*testing.T) {
	return func(t *testing.T) {
		trie := NewTrie[any]()
		opSequence = append(opSequence, ot.ops...)
		for _, op := range opSequence {
			op(t, trie)
		}
		if t.Failed() {
			// All continuations will fail at the same point, so don't bother running them
			return
		}
		for name, continuation := range ot.continuations {
			t.Run(name, continuation.run(opSequence))
		}
	}
}
