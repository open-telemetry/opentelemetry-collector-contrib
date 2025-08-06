// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamaisecurityeventsreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/akamaisecurityeventsreceiver/internal/metadata"
)

func TestParseRuleData(t *testing.T) {
	// Create a test log record with attack data
	log := plog.NewLogRecord()
	attackData := log.Attributes().PutEmptyMap("attackData")

	// Test data with encoded rule information
	attackData.PutStr("rules", "OTUwMDAy%3bOTUwMDA2%3bQ01ELUlOSkVDVElPTi1BTk9NQUxZ")
	attackData.PutStr("ruleMessages", "U3lzdGVtIENvbW1hbmQgQWNjZXNz%3bU3lzdGVtIENvbW1hbmQgSW5qZWN0aW9u%3bQW5vbWFseSBTY29yZSBFeGNlZWRlZCBmb3IgQ29tbWFuZCBJbmplY3Rpb24%3d")
	attackData.PutStr("ruleActions", "YWxlcnQ%3d%3bYWxlcnQ%3d%3bZGVueQ%3d%3d")

	rules := parseRuleData(log)
	require.Equal(t, 3, rules.Len(), "Expected 3 parsed rules")

	// Verify first rule
	firstRule := rules.At(0).Map()
	ruleVal, exists := firstRule.Get("rule")
	assert.True(t, exists)
	assert.Equal(t, "950002", ruleVal.Str())

	messageVal, exists := firstRule.Get("ruleMessage")
	assert.True(t, exists)
	assert.Equal(t, "System Command Access", messageVal.Str())

	actionVal, exists := firstRule.Get("ruleAction")
	assert.True(t, exists)
	assert.Equal(t, "alert", actionVal.Str())

	// Verify second rule
	secondRule := rules.At(1).Map()
	ruleVal, exists = secondRule.Get("rule")
	assert.True(t, exists)
	assert.Equal(t, "950006", ruleVal.Str())

	messageVal, exists = secondRule.Get("ruleMessage")
	assert.True(t, exists)
	assert.Equal(t, "System Command Injection", messageVal.Str())

	actionVal, exists = secondRule.Get("ruleAction")
	assert.True(t, exists)
	assert.Equal(t, "alert", actionVal.Str())

	// Verify third rule
	thirdRule := rules.At(2).Map()
	ruleVal, exists = thirdRule.Get("rule")
	assert.True(t, exists)
	assert.Equal(t, "CMD-INJECTION-ANOMALY", ruleVal.Str())

	messageVal, exists = thirdRule.Get("ruleMessage")
	assert.True(t, exists)
	assert.Equal(t, "Anomaly Score Exceeded for Command Injection", messageVal.Str())

	actionVal, exists = thirdRule.Get("ruleAction")
	assert.True(t, exists)
	assert.Equal(t, "deny", actionVal.Str())
}

func TestParseRuleDataEmptyAttackData(t *testing.T) {
	log := plog.NewLogRecord()
	rules := parseRuleData(log)
	assert.Equal(t, 0, rules.Len(), "Expected no rules when attackData is missing")
}

func TestParseRuleDataNoRuleFields(t *testing.T) {
	log := plog.NewLogRecord()
	attackData := log.Attributes().PutEmptyMap("attackData")
	attackData.PutStr("clientIP", "192.168.1.1")

	rules := parseRuleData(log)
	assert.Equal(t, 0, rules.Len(), "Expected no rules when rule fields are missing")
}

func TestScraperIntegration(t *testing.T) {
	// Load sample event data
	sampleData := loadSampleEvent(t)

	// Create mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authentication headers are present
		authHeader := r.Header.Get("Authorization")
		assert.NotEmpty(t, authHeader, "Authorization header should be present")

		// Return sample event followed by offset
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s\n", sampleData)
		fmt.Fprintf(w, `{"offset": "test-offset-123"}`)
	}))
	defer mockServer.Close()

	// Create test configuration
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: mockServer.URL,
			Timeout:  5 * time.Second,
		},
		ConfigIDs:       "12345",
		ClientToken:     "test-token",
		ClientSecret:    "test-secret",
		AccessToken:     "test-access",
		Limit:           1000,
		ParseRuleData:   true,
		FlattenRuleData: false,
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	scraper := newAkamaiSecurityEventsScraper(settings, cfg)

	// Start the scraper
	ctx := context.Background()
	err := scraper.start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { _ = scraper.shutdown(ctx) }()

	// Execute scrape
	logs, err := scraper.scrape(ctx)
	require.NoError(t, err)
	require.Positive(t, logs.LogRecordCount(), "Expected at least one log record")

	// Verify log record content
	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	// Check that main attributes are preserved
	attackData, exists := logRecord.Attributes().Get("attackData")
	assert.True(t, exists, "attackData should be present")
	assert.Equal(t, pcommon.ValueTypeMap, attackData.Type())

	// Check that rule data was parsed
	ruleData, exists := logRecord.Attributes().Get("parsedRuleData")
	assert.True(t, exists, "parsedRuleData should be present")
	assert.Equal(t, pcommon.ValueTypeSlice, ruleData.Type())
	assert.Positive(t, ruleData.Slice().Len(), "Should have parsed rules")

	// Check timestamp extraction
	assert.NotEqual(t, pcommon.Timestamp(0), logRecord.Timestamp(), "Timestamp should be set")
}

func TestScraperWithFlattenedRuleData(t *testing.T) {
	// Load sample event data
	sampleData := loadSampleEvent(t)

	// Create mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s\n", sampleData)
		fmt.Fprintf(w, `{"offset": "test-offset-123"}`)
	}))
	defer mockServer.Close()

	// Create test configuration with flattened rule data
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: mockServer.URL,
			Timeout:  5 * time.Second,
		},
		ConfigIDs:       "12345",
		ClientToken:     "test-token",
		ClientSecret:    "test-secret",
		AccessToken:     "test-access",
		Limit:           1000,
		ParseRuleData:   true,
		FlattenRuleData: true, // Enable flattening
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	scraper := newAkamaiSecurityEventsScraper(settings, cfg)

	// Start the scraper
	ctx := context.Background()
	err := scraper.start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { _ = scraper.shutdown(ctx) }()

	// Execute scrape
	logs, err := scraper.scrape(ctx)
	require.NoError(t, err)

	// With flattened rule data, we should have multiple log records (one per rule)
	logRecordCount := logs.LogRecordCount()
	assert.Greater(t, logRecordCount, 1, "Expected multiple log records when flattening rules")

	// Verify each log record has flattened rule data
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				// Each record should have parsedRuleData as a map (not slice)
				ruleData, exists := logRecord.Attributes().Get("parsedRuleData")
				assert.True(t, exists, "parsedRuleData should be present")
				assert.Equal(t, pcommon.ValueTypeMap, ruleData.Type(), "parsedRuleData should be a map when flattened")
			}
		}
	}
}

func loadSampleEvent(t *testing.T) string {
	t.Helper()

	// Load data from testdata/sample_event.json
	filePath := filepath.Join("testdata", "sample_event.json")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read sample event file")

	// Validate JSON
	var temp any
	err = json.Unmarshal(data, &temp)
	require.NoError(t, err, "Sample event data should be valid JSON")

	return string(data)
}
