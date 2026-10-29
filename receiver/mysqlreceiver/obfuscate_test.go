// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObfuscateSQLError(t *testing.T) {
	// An unterminated string literal causes ObfuscateSQLStringWithOptions to return an error.
	_, err := newObfuscator().obfuscateSQLString("SELECT 'unterminated")
	assert.Error(t, err)
}

func TestObfuscatePlanError(t *testing.T) {
	// Malformed JSON causes ObfuscateSQLExecPlan to return an error.
	_, err := newObfuscator().obfuscatePlan("{invalid json")
	assert.Error(t, err)
}

func TestObfuscateSQL(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "obfuscate", "expectedSQL.sql"))
	assert.NoError(t, err)
	expectedSQL := strings.TrimSpace(string(expected))

	input, err := os.ReadFile(filepath.Join("testdata", "obfuscate", "inputSQL.sql"))
	assert.NoError(t, err)

	result, err := newObfuscator().obfuscateSQLString(string(input))
	assert.NoError(t, err)
	assert.Equal(t, expectedSQL, result)
}

// runPlanTests is a helper that drives table-driven tests for plan obfuscation.
// planFunc is the method under test.
func runPlanTests(t *testing.T, planFunc func(string) (string, error), tests []struct {
	name         string
	inputFile    string
	expectedFile string
},
) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "obfuscate", tc.inputFile))
			require.NoError(t, err)

			result, err := planFunc(string(input))
			require.NoError(t, err)

			expected, err := os.ReadFile(filepath.Join("testdata", "obfuscate", tc.expectedFile))
			require.NoError(t, err)

			// Normalize JSON for comparison to ignore formatting differences
			var resultJSON, expectedJSON any
			require.NoError(t, json.Unmarshal([]byte(result), &resultJSON))
			require.NoError(t, json.Unmarshal(expected, &expectedJSON))

			assert.Equal(t, expectedJSON, resultJSON)
		})
	}
}

func TestObfuscatePlan(t *testing.T) {
	// MySQL 8.4 EXPLAIN FORMAT=JSON produces two formats:
	//   Version 1 (default, explain_json_format_version=1): query_block → ordering_operation → table → attached_condition
	//   Version 2 (explain_json_format_version=2):          query + inputs array, each node has condition/operation/access_type
	// Fixtures in testdata/obfuscate/ were captured from a MySQL 8.4 instance using EXPLAIN FORMAT=JSON.
	runPlanTests(t, newObfuscator().obfuscatePlan, []struct {
		name         string
		inputFile    string
		expectedFile string
	}{
		{
			name:         "version1_query_block",
			inputFile:    "inputQueryPlan.json",
			expectedFile: "expectedQueryPlan.json",
		},
		{
			name:         "version2_inputs_array",
			inputFile:    "inputQueryPlanV2.json",
			expectedFile: "expectedQueryPlanV2.json",
		},
	})
}
