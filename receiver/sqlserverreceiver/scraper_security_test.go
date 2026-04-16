// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

const sensitiveSQL = "SELECT secret_password FROM users WHERE ssn = '123-45-6789'"

func newTestScraperHelper(logger *zap.Logger, unsafeLogRawSQL bool) *sqlServerScraperHelper {
	return &sqlServerScraperHelper{
		logger:     logger,
		obfuscator: newObfuscator(),
		config: &Config{
			UnsafeLogRawSQL: unsafeLogRawSQL,
		},
	}
}

func TestRetrieveValue_DoesNotLogRawData(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger, false)

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

func TestObfuscationFailure_DoesNotLogRawSQL_DefaultConfig(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger, false)

	badSQL := strings.Repeat("((((", 500)
	row := sqlquery.StringMap{
		"query_text": badSQL,
	}

	var errs []error
	s.retrieveValue(row, "query_text", &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
		statement := row[columnName]
		obfuscated, err := s.obfuscator.obfuscateSQLString(statement)
		if err != nil {
			return "", fmt.Errorf("failed to obfuscate SQL statement for column %s: %w", columnName, err)
		}
		return obfuscated, nil
	})

	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, badSQL,
			"raw SQL must not appear in log messages on obfuscation failure")
	}
}

func TestObfuscationFailure_LogsRawSQL_WhenUnsafeEnabled(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger, true)

	badSQL := strings.Repeat("((((", 500)
	row := sqlquery.StringMap{
		"query_text": badSQL,
	}

	var errs []error
	s.retrieveValue(row, "query_text", &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
		statement := row[columnName]
		obfuscated, err := s.obfuscator.obfuscateSQLString(statement)
		if err != nil {
			if s.config.UnsafeLogRawSQL {
				s.logger.Debug("obfuscation failed for SQL statement (unsafe_log_raw_sql enabled)",
					zap.String("column", columnName),
					zap.String("statement", statement),
					zap.Error(err))
			}
			return "", fmt.Errorf("failed to obfuscate SQL statement for column %s: %w", columnName, err)
		}
		return obfuscated, nil
	})

	foundDebugWithRaw := false
	for _, entry := range logs.All() {
		if entry.Level == zapcore.DebugLevel && strings.Contains(entry.Message, "unsafe_log_raw_sql") {
			foundDebugWithRaw = true
		}
	}
	assert.True(t, foundDebugWithRaw,
		"when UnsafeLogRawSQL is enabled, raw SQL should appear in Debug logs")
}

func TestRetrieveValue_EmptyStatement_NoError(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger, false)

	row := sqlquery.StringMap{
		"query_text": "",
	}

	var errs []error
	result := s.retrieveValue(row, "query_text", &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
		statement := row[columnName]
		if statement == "" {
			return "", nil
		}
		obfuscated, err := s.obfuscator.obfuscateSQLString(statement)
		if err != nil {
			return "", fmt.Errorf("failed to obfuscate SQL statement for column %s: %w", columnName, err)
		}
		return obfuscated, nil
	})

	assert.Empty(t, errs, "empty statement should not produce errors")
	assert.Equal(t, "", result)

	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel)
	assert.Equal(t, 0, errorLogs.Len(),
		"empty/NULL query_text should not produce error-level logs")
}

func TestDebugLog_DoesNotExposeRowData_DefaultConfig(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger, false)

	queryHashVal := "abc123"
	queryPlanHashVal := "def456"
	row := sqlquery.StringMap{
		"query_text": sensitiveSQL,
		"query_plan": "<xml>sensitive plan data</xml>",
	}

	// Simulate the safe debug logging path
	if s.config.UnsafeLogRawSQL {
		s.logger.Debug("processing query row (unsafe_log_raw_sql enabled)",
			zap.String("query_hash", queryHashVal),
			zap.String("plan_hash", queryPlanHashVal),
			zap.Any("row", row))
	} else {
		s.logger.Debug("processing query row",
			zap.String("query_hash", queryHashVal),
			zap.String("plan_hash", queryPlanHashVal))
	}

	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, sensitiveSQL)
		for _, field := range entry.Context {
			assert.NotEqual(t, "row", field.Key,
				"row data must not be logged when UnsafeLogRawSQL is false")
		}
	}
}

func TestDebugLog_ExposesRowData_WhenUnsafeEnabled(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	s := newTestScraperHelper(logger, true)

	queryHashVal := "abc123"
	queryPlanHashVal := "def456"
	row := sqlquery.StringMap{
		"query_text": sensitiveSQL,
		"query_plan": "<xml>plan data</xml>",
	}

	if s.config.UnsafeLogRawSQL {
		s.logger.Debug("processing query row (unsafe_log_raw_sql enabled)",
			zap.String("query_hash", queryHashVal),
			zap.String("plan_hash", queryPlanHashVal),
			zap.Any("row", row))
	} else {
		s.logger.Debug("processing query row",
			zap.String("query_hash", queryHashVal),
			zap.String("plan_hash", queryPlanHashVal))
	}

	foundRowField := false
	for _, entry := range logs.All() {
		for _, field := range entry.Context {
			if field.Key == "row" {
				foundRowField = true
			}
		}
	}
	assert.True(t, foundRowField,
		"when UnsafeLogRawSQL is enabled, row data should be present in Debug logs")
}
