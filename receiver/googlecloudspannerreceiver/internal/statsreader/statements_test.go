// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statsreader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	query = "query"

	topMetricsQueryMaxRows = 10
)

func TestCurrentStatsStatement(t *testing.T) {
	testCases := map[string]struct {
		topMetricsQueryMaxRows int
		expectedSQL            string
		expectedParams         map[string]interface{}
	}{
		"Statement with top metrics query max rows":    {topMetricsQueryMaxRows, query + topMetricsQueryLimitCondition, map[string]interface{}{topMetricsQueryLimitParameterName: topMetricsQueryMaxRows}},
		"Statement without top metrics query max rows": {0, query, map[string]interface{}{}},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			args := statementArgs{
				query:                  query,
				topMetricsQueryMaxRows: testCase.topMetricsQueryMaxRows,
			}

			statement := currentStatsStatement(args)

			assert.Equal(t, testCase.expectedSQL, statement.SQL)
			assert.Equal(t, testCase.expectedParams, statement.Params)
		})
	}
}

func TestIntervalStatsStatement(t *testing.T) {
	pullTimestamp := time.Now().UTC()

	testCases := map[string]struct {
		topMetricsQueryMaxRows int
		expectedSQL            string
		expectedParams         map[string]interface{}
	}{
		"Statement with top metrics query max rows": {topMetricsQueryMaxRows, query + topMetricsQueryLimitCondition, map[string]interface{}{
			topMetricsQueryLimitParameterName: topMetricsQueryMaxRows,
			pullTimestampParameterName:        pullTimestamp,
		}},
		"Statement without top metrics query max rows": {0, query, map[string]interface{}{pullTimestampParameterName: pullTimestamp}},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			args := statementArgs{
				query:                  query,
				topMetricsQueryMaxRows: testCase.topMetricsQueryMaxRows,
				pullTimestamp:          pullTimestamp,
			}

			statement := intervalStatsStatement(args)

			assert.Equal(t, testCase.expectedSQL, statement.SQL)
			assert.Equal(t, testCase.expectedParams, statement.Params)
		})
	}
}
