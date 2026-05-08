// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// TODO: remove build constraint once https://github.com/pavolloffay/opentelemetry-mcp-server/issues/... is fixed
// (collectorschema uses filepath.Join with embed.FS, which produces backslash paths on Windows)
//go:build !windows

package tools

import (
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pavolloffay/opentelemetry-mcp-server/modules/collectorschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSchemaManager(t *testing.T) (*collectorschema.SchemaManager, string) {
	t.Helper()
	sm := collectorschema.NewSchemaManager()
	version, err := sm.GetLatestVersion()
	require.NoError(t, err)
	return sm, version
}

func newRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func versionWithChangelog(t *testing.T, sm *collectorschema.SchemaManager) string {
	t.Helper()
	versions, err := sm.GetAllVersions()
	require.NoError(t, err)
	for _, version := range slices.Backward(versions) {
		changelog, err := sm.GetChangelog(version)
		if err == nil && changelog != "" {
			return version
		}
	}
	t.Skip("no version with changelog found in embedded data")
	return ""
}

func TestGetAllTools(t *testing.T) {
	tools, err := GetAllTools()
	require.NoError(t, err)
	assert.Len(t, tools, 8)

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Tool.Name)
	}
	assert.ElementsMatch(t, []string{
		"opentelemetry-collector-get-versions",
		"opentelemetry-collector-components",
		"opentelemetry-collector-readme",
		"opentelemetry-collector-component-schema",
		"opentelemetry-collector-component-schema-validation",
		"opentelemetry-collector-component-deprecated-fields",
		"opentelemetry-collector-changelog",
		"opentelemetry-collector-rag",
	}, names)
}

func TestCollectorVersionsTool(t *testing.T) {
	sm, _ := setupSchemaManager(t)
	tool := getCollectorVersionsTool(sm)

	result, err := tool.Handler(t.Context(), newRequest(nil))
	require.NoError(t, err)
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "versions:")
}

func TestCollectorComponentsTool(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorComponentsTool(sm, version)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
	}{
		{
			name:        "missing required kind",
			wantIsError: true,
		},
		{
			name:        "valid kind",
			args:        map[string]any{"kind": "receiver"},
			wantIsError: false,
		},
		{
			name:        "valid kind with explicit version",
			args:        map[string]any{"kind": "exporter", "version": version},
			wantIsError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
		})
	}
}

func TestCollectorReadmeTool(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorReadmeTool(sm, version)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
	}{
		{
			name:        "missing required kind",
			wantIsError: true,
		},
		{
			name:        "missing required name",
			args:        map[string]any{"kind": "receiver"},
			wantIsError: true,
		},
		{
			name:        "valid kind and name",
			args:        map[string]any{"kind": "receiver", "name": "otlp"},
			wantIsError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
		})
	}
}

func TestCollectorChangelogTool(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorChangelogTool(sm, version)
	changelogVersion := versionWithChangelog(t, sm)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
	}{
		{
			name:        "version without changelog returns error result",
			args:        map[string]any{"version": "0.0.0"},
			wantIsError: true,
		},
		{
			name:        "version with changelog succeeds",
			args:        map[string]any{"version": changelogVersion},
			wantIsError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
		})
	}
}

func TestCollectorSchemaGetTool(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorSchemaGetTool(sm, version)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
	}{
		{
			name:        "missing required kind",
			wantIsError: true,
		},
		{
			name:        "missing required name",
			args:        map[string]any{"kind": "receiver"},
			wantIsError: true,
		},
		{
			name:        "valid kind and name",
			args:        map[string]any{"kind": "receiver", "name": "otlp"},
			wantIsError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
		})
	}
}

func TestCollectorSchemaValidationTool(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorSchemaValidationTool(sm, version)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
		checkResult func(t *testing.T, result *mcp.CallToolResult)
	}{
		{
			name:        "missing required kind",
			wantIsError: true,
		},
		{
			name:        "missing required name",
			args:        map[string]any{"kind": "receiver"},
			wantIsError: true,
		},
		{
			name:        "missing required config",
			args:        map[string]any{"kind": "receiver", "name": "otlp"},
			wantIsError: true,
		},
		{
			name:        "valid config",
			args:        map[string]any{"kind": "receiver", "name": "otlp", "config": `{"protocols": {"grpc": {}}}`},
			wantIsError: false,
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				text, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok)
				assert.Contains(t, text.Text, "is valid:")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestCollectorComponentDeprecatedTool(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorComponentDeprecatedTool(sm, version)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
	}{
		{
			name:        "missing required kind",
			wantIsError: true,
		},
		{
			name:        "missing required names",
			args:        map[string]any{"kind": "receiver"},
			wantIsError: true,
		},
		{
			name:        "valid kind and names",
			args:        map[string]any{"kind": "receiver", "names": []any{"otlp"}},
			wantIsError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
		})
	}
}

func TestCollectorDocumentationRAG(t *testing.T) {
	sm, version := setupSchemaManager(t)
	tool := getCollectorDocumentationRAG(sm, version)

	tests := []struct {
		name        string
		args        map[string]any
		wantIsError bool
	}{
		{
			name:        "missing required version",
			args:        map[string]any{"query": "how to configure otlp receiver"},
			wantIsError: true,
		},
		{
			name:        "missing required query",
			args:        map[string]any{"version": version},
			wantIsError: true,
		},
		{
			name:        "valid query without component filter",
			args:        map[string]any{"query": "how to configure otlp receiver", "version": version},
			wantIsError: false,
		},
		{
			name:        "valid query with component filter",
			args:        map[string]any{"query": "endpoints", "version": version, "kind": "receiver", "name": "otlp"},
			wantIsError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(t.Context(), newRequest(tt.args))
			require.NoError(t, err)
			assert.Equal(t, tt.wantIsError, result.IsError)
		})
	}
}
