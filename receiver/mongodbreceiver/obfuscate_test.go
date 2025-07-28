// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestObfuscateCommand(t *testing.T) {
	o := newObfuscator()

	command := bson.D{
		{Key: "find", Value: "users"},
		{Key: "filter", Value: bson.M{"name": "test", "age": 30}},
		{Key: "comment", Value: "test query"},
	}

	cleanedCommand := cleanCommand(command)
	obfuscated := o.obfuscateSQLString(cleanedCommand.String())
	require.Contains(t, obfuscated, "find")
	require.NotContains(t, obfuscated, "users")
	require.NotContains(t, obfuscated, "test")
	require.NotContains(t, obfuscated, "30")
}

func TestGenerateQuerySignature(t *testing.T) {
	query1 := `{"find":"users","filter":{"name":"test"}}`
	query2 := `{"find":"users","filter":{"name":"different"}}`

	sig1 := generateQuerySignature(query1)
	sig2 := generateQuerySignature(query2)
	sig1Again := generateQuerySignature(query1)

	require.Equal(t, sig1, sig1Again)
	require.NotEqual(t, sig1, sig2)
	require.Len(t, sig1, 16) // 8 bytes hex encoded
}

func TestObfuscateExplainPlan(t *testing.T) {
	tests := []struct {
		name     string
		plan     any
		expected any
	}{
		{
			name: "simple explain plan with filter",
			plan: bson.M{
				"queryPlanner": bson.M{
					"filter": bson.M{
						"field1": "value1",
					},
				},
			},
			expected: bson.M{
				"queryPlanner": bson.M{
					"filter": bson.M{
						"field1": "?",
					},
				},
			},
		},
		{
			name: "simple explain plan with parsedQuery",
			plan: bson.M{
				"queryPlanner": bson.M{
					"parsedQuery": bson.M{
						"field1": "value1",
					},
				},
			},
			expected: bson.M{
				"queryPlanner": bson.M{
					"parsedQuery": bson.M{
						"field1": "?",
					},
				},
			},
		},
		{
			name: "simple explain plan with indexBounds",
			plan: bson.M{
				"queryPlanner": bson.M{
					"indexBounds": bson.M{
						"field1": "value1",
					},
				},
			},
			expected: bson.M{
				"queryPlanner": bson.M{
					"indexBounds": bson.M{
						"field1": "?",
					},
				},
			},
		},
		{
			name: "nested explain plan",
			plan: bson.M{
				"queryPlanner": bson.M{
					"winningPlan": bson.M{
						"inputStage": bson.M{
							"filter": bson.M{
								"field1": "value1",
							},
						},
					},
				},
			},
			expected: bson.M{
				"queryPlanner": bson.M{
					"winningPlan": bson.M{
						"inputStage": bson.M{
							"filter": bson.M{
								"field1": "?",
							},
						},
					},
				},
			},
		},
		{
			name: "explain plan with different data types",
			plan: bson.M{
				"queryPlanner": bson.M{
					"filter": bson.M{
						"field1": "value1",
						"field2": 123,
						"field3": true,
					},
				},
			},
			expected: bson.M{
				"queryPlanner": bson.M{
					"filter": bson.M{
						"field1": "?",
						"field2": "?",
						"field3": "?",
					},
				},
			},
		},
		{
			name:     "empty explain plan",
			plan:     bson.M{},
			expected: bson.M{},
		},
		{
			name: "explain plan with no fields to obfuscate",
			plan: bson.M{
				"queryPlanner": bson.M{
					"winningPlan": bson.M{
						"stage": "COLLSCAN",
					},
				},
			},
			expected: bson.M{
				"queryPlanner": bson.M{
					"winningPlan": bson.M{
						"stage": "COLLSCAN",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := obfuscateExplainPlan(tt.plan)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestPrepareCommandForExplain(t *testing.T) {
	command := bson.D{
		{Key: "find", Value: "users"},
		{Key: "$db", Value: "test"},
		{Key: "readConcern", Value: "majority"},
	}
	prepared := prepareCommandForExplain(command)
	require.Len(t, prepared, 1)
	require.Equal(t, "find", prepared[0].Key)
}

func TestCleanCommand(t *testing.T) {
	command := bson.D{
		{Key: "find", Value: "users"},
		{Key: "comment", Value: "test comment"},
		{Key: "lsid", Value: "some-id"},
	}
	cleaned := cleanCommand(command)
	require.Len(t, cleaned, 1)
	require.Equal(t, "find", cleaned[0].Key)
}

func TestObfuscateLiterals(t *testing.T) {
	value := bson.M{
		"field1": "value1",
		"field2": 123,
		"nested": bson.M{
			"field3": true,
		},
	}
	obfuscated := obfuscateLiterals(value)
	expected := bson.M{
		"field1": "?",
		"field2": "?",
		"nested": bson.M{
			"field3": "?",
		},
	}
	require.Equal(t, expected, obfuscated)
}

func TestCleanExplainPlan(t *testing.T) {
	plan := bson.M{
		"queryPlanner": bson.M{},
		"serverInfo":   bson.M{},
		"ok":           1,
	}
	cleaned := cleanExplainPlan(plan)
	require.Len(t, cleaned, 1)
	require.Contains(t, cleaned, "queryPlanner")
}
