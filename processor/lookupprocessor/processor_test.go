// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
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
		Lookups: []LookupConfig{
			{
				Key:     `log.attributes["user.id"]`,
				Context: ContextRecord,
				Attributes: []AttributeMapping{
					{
						Destination: "user.name",
						Default:     "Unknown User",
					},
				},
			},
			{
				Key:     `resource.attributes["service.name"]`,
				Context: ContextResource,
				Attributes: []AttributeMapping{
					{Destination: "service.display_name"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc-frontend")

	sl := rl.ScopeLogs().AppendEmpty()

	log1 := sl.LogRecords().AppendEmpty()
	log1.Body().SetStr("User logged in")
	log1.Attributes().PutStr("user.id", "user001")

	log2 := sl.LogRecords().AppendEmpty()
	log2.Body().SetStr("User performed action")
	log2.Attributes().PutStr("user.id", "user999")

	log3 := sl.LogRecords().AppendEmpty()
	log3.Body().SetStr("User logged out")
	log3.Attributes().PutStr("user.id", "user002")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	require.Len(t, sink.AllLogs(), 1)
	processedLogs := sink.AllLogs()[0]

	resource := processedLogs.ResourceLogs().At(0).Resource()
	serviceName, ok := resource.Attributes().Get("service.display_name")
	assert.True(t, ok, "service.display_name should be set")
	assert.Equal(t, "Frontend Web App", serviceName.Str())

	logRecords := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 3, logRecords.Len())

	userName1, ok := logRecords.At(0).Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Alice Johnson", userName1.Str())

	userName2, ok := logRecords.At(1).Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Unknown User", userName2.Str())

	userName3, ok := logRecords.At(2).Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Bob Smith", userName3.Str())
}

func TestProcessorMapResult(t *testing.T) {
	// Source returns map[string]any for 1:N lookups
	mapSource := lookupsource.NewSourceFactory(
		"mapresult",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, key string) (any, bool, error) {
					if key == "user001" {
						return map[string]any{
							"name":  "Alice Johnson",
							"email": "alice@example.com",
							"role":  "admin",
						}, true, nil
					}
					return nil, false, nil
				},
				func() string { return "mapresult" },
				nil,
				nil,
			), nil
		},
	)

	factory := NewFactoryWithOptions(WithSources(mapSource))
	cfg := &Config{
		Source: SourceConfig{Type: "mapresult"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["user.id"]`,
				Attributes: []AttributeMapping{
					{Source: "name", Destination: "user.name", Default: "Unknown"},
					{Source: "email", Destination: "user.email"},
					{Source: "role", Destination: "user.role", Context: ContextResource},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	// Test with found key
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Attributes().PutStr("user.id", "user001")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	processedLogs := sink.AllLogs()[0]
	record := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	resourceAttrs := processedLogs.ResourceLogs().At(0).Resource().Attributes()

	name, ok := record.Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Alice Johnson", name.Str())

	email, ok := record.Attributes().Get("user.email")
	assert.True(t, ok)
	assert.Equal(t, "alice@example.com", email.Str())

	role, ok := resourceAttrs.Get("user.role")
	assert.True(t, ok)
	assert.Equal(t, "admin", role.Str())
}

func TestProcessorMapResultNotFound(t *testing.T) {
	mapSource := lookupsource.NewSourceFactory(
		"mapresult",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return nil, false, nil
				},
				func() string { return "mapresult" },
				nil,
				nil,
			), nil
		},
	)

	factory := NewFactoryWithOptions(WithSources(mapSource))
	cfg := &Config{
		Source: SourceConfig{Type: "mapresult"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["user.id"]`,
				Attributes: []AttributeMapping{
					{Source: "name", Destination: "user.name", Default: "Unknown"},
					{Source: "email", Destination: "user.email"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Attributes().PutStr("user.id", "unknown-user")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	processedLogs := sink.AllLogs()[0]
	record := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	// Default should be applied for "name"
	name, ok := record.Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Unknown", name.Str())

	// No default for "email" — should not be set
	_, ok = record.Attributes().Get("user.email")
	assert.False(t, ok)
}

func TestProcessorNoSourceAttribute(t *testing.T) {
	factory := NewFactoryWithOptions(WithSources(mockSourceFactory("found")))

	cfg := &Config{
		Source: SourceConfig{Type: "mock"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["nonexistent"]`,
				Attributes: []AttributeMapping{
					{Destination: "result"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
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
