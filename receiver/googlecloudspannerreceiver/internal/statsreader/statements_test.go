// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
				stalenessRead:          true,
			}

			stmt := currentStatsStatement(args)

			assert.Equal(t, testCase.expectedSQL, stmt.statement.SQL)
			assert.Equal(t, testCase.expectedParams, stmt.statement.Params)
			assert.True(t, stmt.stalenessRead)
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
				stalenessRead:          true,
			}

			stmt := intervalStatsStatement(args)

			assert.Equal(t, testCase.expectedSQL, stmt.statement.SQL)
			assert.Equal(t, testCase.expectedParams, stmt.statement.Params)
			assert.True(t, stmt.stalenessRead)
		})
	}
}
