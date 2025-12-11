// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func TestProcessorLookup(t *testing.T) {
	// Create a mock source with test mappings
	mappings := map[string]any{
		"user001":      "Alice Johnson",
		"user002":      "Bob Smith",
		"user003":      "Charlie Brown",
		"svc-frontend": "Frontend Web App",
		"svc-backend":  "Backend API Service",
	}

	factory := NewFactoryWithOptions(WithSources(mockMapSourceFactory(mappings)))
	cfg := &Config{
		Source: SourceConfig{Type: "mockmap"},
		Attributes: []AttributeConfig{
			{
				Key:           "user.name",
				FromAttribute: "user.id",
				Default:       "Unknown User",
				Action:        ActionUpsert,
				Context:       ContextRecord,
			},
			{
				Key:           "service.display_name",
				FromAttribute: "service.name",
				Action:        ActionInsert,
				Context:       ContextResource,
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	// Start the processor
	host := &testHost{}
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	// Create test logs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc-frontend")

	sl := rl.ScopeLogs().AppendEmpty()

	// Log 1: user001 should resolve to "Alice Johnson"
	log1 := sl.LogRecords().AppendEmpty()
	log1.Body().SetStr("User logged in")
	log1.Attributes().PutStr("user.id", "user001")

	// Log 2: user999 should get default "Unknown User"
	log2 := sl.LogRecords().AppendEmpty()
	log2.Body().SetStr("User performed action")
	log2.Attributes().PutStr("user.id", "user999")

	// Log 3: user002 should resolve to "Bob Smith"
	log3 := sl.LogRecords().AppendEmpty()
	log3.Body().SetStr("User logged out")
	log3.Attributes().PutStr("user.id", "user002")

	// Process the logs
	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Verify the results
	require.Len(t, sink.AllLogs(), 1)
	processedLogs := sink.AllLogs()[0]

	// Check resource attributes
	resource := processedLogs.ResourceLogs().At(0).Resource()
	serviceName, ok := resource.Attributes().Get("service.display_name")
	assert.True(t, ok, "service.display_name should be added")
	assert.Equal(t, "Frontend Web App", serviceName.Str())

	// Check log record attributes
	logRecords := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 3, logRecords.Len())

	// Log 1: user001 -> "Alice Johnson"
	userName1, ok := logRecords.At(0).Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Alice Johnson", userName1.Str())

	// Log 2: user999 -> "Unknown User" (default)
	userName2, ok := logRecords.At(1).Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Unknown User", userName2.Str())

	// Log 3: user002 -> "Bob Smith"
	userName3, ok := logRecords.At(2).Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Bob Smith", userName3.Str())
}

func TestProcessorActions(t *testing.T) {
	// Create a mock source that always returns "found"
	factory := NewFactoryWithOptions(WithSources(mockSourceFactory("found")))

	tests := []struct {
		name           string
		action         Action
		existingValue  string
		expectedValue  string
		expectModified bool
	}{
		{
			name:           "insert on new attribute",
			action:         ActionInsert,
			existingValue:  "",
			expectedValue:  "found",
			expectModified: true,
		},
		{
			name:           "insert on existing attribute - no change",
			action:         ActionInsert,
			existingValue:  "existing",
			expectedValue:  "existing",
			expectModified: false,
		},
		{
			name:           "update on existing attribute",
			action:         ActionUpdate,
			existingValue:  "existing",
			expectedValue:  "found",
			expectModified: true,
		},
		{
			name:           "update on new attribute - no change",
			action:         ActionUpdate,
			existingValue:  "",
			expectedValue:  "",
			expectModified: false,
		},
		{
			name:           "upsert on new attribute",
			action:         ActionUpsert,
			existingValue:  "",
			expectedValue:  "found",
			expectModified: true,
		},
		{
			name:           "upsert on existing attribute",
			action:         ActionUpsert,
			existingValue:  "existing",
			expectedValue:  "found",
			expectModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Source: SourceConfig{Type: "mock"},
				Attributes: []AttributeConfig{
					{
						Key:           "result",
						FromAttribute: "key",
						Action:        tt.action,
						Context:       ContextRecord,
					},
				},
			}

			sink := &consumertest.LogsSink{}
			settings := processortest.NewNopSettings(metadata.Type)

			proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
			require.NoError(t, err)

			host := &testHost{}
			require.NoError(t, proc.Start(t.Context(), host))
			defer func() { _ = proc.Shutdown(t.Context()) }()

			// Create test log
			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			lr.Attributes().PutStr("key", "lookup-key")
			if tt.existingValue != "" {
				lr.Attributes().PutStr("result", tt.existingValue)
			}

			err = proc.ConsumeLogs(t.Context(), logs)
			require.NoError(t, err)

			processedLogs := sink.AllLogs()[0]
			resultAttr, exists := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("result")

			if tt.expectedValue == "" {
				assert.False(t, exists, "result attribute should not exist")
			} else {
				assert.True(t, exists, "result attribute should exist")
				assert.Equal(t, tt.expectedValue, resultAttr.Str())
			}
		})
	}
}

func TestProcessorNoSourceAttribute(t *testing.T) {
	factory := NewFactoryWithOptions(WithSources(mockSourceFactory("found")))

	cfg := &Config{
		Source: SourceConfig{Type: "mock"},
		Attributes: []AttributeConfig{
			{
				Key:           "result",
				FromAttribute: "nonexistent",
				Action:        ActionUpsert,
				Context:       ContextRecord,
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := &testHost{}
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	// Create test log without the source attribute
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Attributes().PutStr("other", "value")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Result should not be set since source attribute doesn't exist
	processedLogs := sink.AllLogs()[0]
	_, exists := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("result")
	assert.False(t, exists, "result should not be set when source attribute is missing")
}

// mockSourceFactory creates a source factory that always returns the given value.
func mockSourceFactory(value string) lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		"mock",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return value, true, nil
				},
				func() string { return "mock" },
				nil,
				nil,
			), nil
		},
	)
}

// mockMapSourceFactory creates a source factory that looks up values from a map.
func mockMapSourceFactory(mappings map[string]any) lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		"mockmap",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, key string) (any, bool, error) {
					val, found := mappings[key]
					return val, found, nil
				},
				func() string { return "mockmap" },
				nil,
				nil,
			), nil
		},
	)
}
