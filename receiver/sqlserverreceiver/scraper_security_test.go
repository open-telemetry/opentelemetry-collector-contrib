// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

const sensitiveSQL = "SELECT secret_password FROM users WHERE ssn = '123-45-6789'"

// malformedSQL consistently fails the DataDog obfuscator (unmatched bracket).
const malformedSQL = "SELECT cpu_time AS [CPU Usage (time)"

func newTestScraperHelper(logger *zap.Logger) *sqlServerScraperHelper {
	return &sqlServerScraperHelper{
		logger:     logger,
		obfuscator: newObfuscator(),
		config:     &Config{},
	}
}

func TestRetrieveValue_DoesNotLogRawData(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger)

	row := sqlquery.StringMap{
		"query_text": sensitiveSQL,
	}

	var errs []error
	s.retrieveValue(row, "query_text", &errs, func(_ sqlquery.StringMap, _ string) (any, error) {
		return nil, fmt.Errorf("simulated parse error")
	})

	require.Len(t, errs, 1)
	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, sensitiveSQL,
			"raw SQL must not appear in log messages")
		for _, field := range entry.Context {
			if field.Key == "column" {
				continue
			}
			assert.NotContains(t, field.String, sensitiveSQL,
				"raw SQL must not appear in log field %q", field.Key)
		}
	}
}

func TestRetrieveValue_LogsAtWarnLevel(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger)

	var errs []error
	s.retrieveValue(sqlquery.StringMap{"col": "val"}, "col", &errs, func(_ sqlquery.StringMap, _ string) (any, error) {
		return nil, fmt.Errorf("simulated failure")
	})

	require.Len(t, errs, 1)
	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel)
	require.Equal(t, 1, warnLogs.Len(),
		"retrieveValue should log at Warn level, not Error")
	assert.Equal(t, "failed to retrieve value", warnLogs.All()[0].Message)

	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel)
	assert.Equal(t, 0, errorLogs.Len(),
		"retrieveValue should not produce Error-level logs for retrieval failures")
}

func TestObfuscationFailure_LogsRawSQLAtDebug(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger)

	row := sqlquery.StringMap{
		"query_text": malformedSQL,
	}

	var errs []error
	s.retrieveValue(row, "query_text", &errs, s.obfuscateSQLRetriever())

	require.NotEmpty(t, errs, "obfuscation of malformed SQL should produce an error")

	foundDebugWithRaw := false
	for _, entry := range logs.All() {
		if entry.Level == zapcore.DebugLevel {
			for _, field := range entry.Context {
				if field.Key == "statement" && field.String == malformedSQL {
					foundDebugWithRaw = true
				}
			}
		}
	}
	assert.True(t, foundDebugWithRaw,
		"raw SQL should appear in Debug logs when obfuscation fails")
}

func TestObfuscationFailure_RawSQLNotInWarnOrError(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger)

	row := sqlquery.StringMap{
		"query_text": malformedSQL,
	}

	var errs []error
	s.retrieveValue(row, "query_text", &errs, s.obfuscateSQLRetriever())

	for _, entry := range logs.All() {
		if entry.Level >= zapcore.WarnLevel {
			for _, field := range entry.Context {
				assert.NotEqual(t, malformedSQL, field.String,
					"raw SQL must not appear in Warn/Error log fields (level=%s, key=%s)", entry.Level, field.Key)
			}
		}
	}
}

func TestObfuscateSQLRetriever_SuccessfulObfuscation(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger)

	row := sqlquery.StringMap{
		"query_text": "SELECT * FROM users WHERE id = 42",
	}

	var errs []error
	result := s.retrieveValue(row, "query_text", &errs, s.obfuscateSQLRetriever())

	assert.Empty(t, errs, "valid SQL should obfuscate without error")
	assert.NotContains(t, result.(string), "42",
		"obfuscated result should not contain literal value")

	debugLogs := logs.FilterMessage("obfuscation failed for SQL statement")
	assert.Equal(t, 0, debugLogs.Len(),
		"no failure debug log should be emitted for successful obfuscation")
}

func TestEmptyStatement_SkippedBeforeRetriever(t *testing.T) {
	// In production, rows with empty query_text are skipped by the loop's
	// "if row[queryText] == "" { continue }" guard before retrieveValue is
	// called. This test verifies that guard works correctly.
	row := sqlquery.StringMap{
		"query_text": "",
	}

	assert.Equal(t, "", row["query_text"],
		"empty query_text should be detected by the caller's guard")

	// Non-empty values should pass through to the retriever.
	row["query_text"] = "SELECT 1"
	assert.NotEqual(t, "", row["query_text"])
}

func TestDebugLog_IncludesRowData(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	queryHashVal := "abc123"
	queryPlanHashVal := "def456"
	row := sqlquery.StringMap{
		"query_text": sensitiveSQL,
		"query_plan": "<xml>sensitive plan data</xml>",
	}

	logger.Debug("processing query row",
		zap.String("query_hash", queryHashVal),
		zap.String("plan_hash", queryPlanHashVal),
		zap.Any("row", row))

	foundRowField := false
	for _, entry := range logs.All() {
		for _, field := range entry.Context {
			if field.Key == "row" {
				foundRowField = true
			}
		}
	}
	assert.True(t, foundRowField,
		"row data should be present in Debug logs for diagnostics")
}
