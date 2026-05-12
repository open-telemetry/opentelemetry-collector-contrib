// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// tools package includes the list of tools available for the MCP extension. The contents of this
// file originated in https://github.com/pavolloffay/opentelemetry-mcp-server/blob/main/internal/tools/tools.go
package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/mcp/tools"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pavolloffay/opentelemetry-mcp-server/modules/collectorschema"
)

// Tool represents an MCP tool with its handler
type Tool struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

// GetAllTools returns a list of all available MCP tools
func GetAllTools() ([]Tool, error) {
	schemaManager := collectorschema.NewSchemaManager()
	latestCollectorVersion, err := schemaManager.GetLatestVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest collector version: %w", err)
	}

	tools := []Tool{
		getCollectorVersionsTool(schemaManager),
		getCollectorComponentsTool(schemaManager, latestCollectorVersion),
		getCollectorReadmeTool(schemaManager, latestCollectorVersion),
		getCollectorSchemaGetTool(schemaManager, latestCollectorVersion),
		getCollectorSchemaValidationTool(schemaManager, latestCollectorVersion),
		getCollectorComponentDeprecatedTool(schemaManager, latestCollectorVersion),
		getCollectorChangelogTool(schemaManager, latestCollectorVersion),
		getCollectorDocumentationRAG(schemaManager, latestCollectorVersion),
	}

	return tools, nil
}

func parseArgs(req *mcp.CallToolRequest) map[string]any {
	var args map[string]any
	if req.Params != nil && len(req.Params.Arguments) > 0 {
		_ = json.Unmarshal(req.Params.Arguments, &args)
	}
	return args
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}

func jsonResult(v any) *mcp.CallToolResult {
	data, err := json.Marshal(v)
	if err != nil {
		return errResult("failed to marshal result: %v", err)
	}
	return textResult(string(data))
}

func stringArg(args map[string]any, key, defaultVal string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}

func requireStringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key].(string)
	return v, ok && v != ""
}

func requireStringSliceArg(args map[string]any, key string) ([]string, bool) {
	raw, ok := args[key].([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result, true
}

func getCollectorVersionsTool(schemaManager *collectorschema.SchemaManager) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-get-versions",
		Description: "Get all supported OpenTelemetry collector versions by this tool",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		versions, err := schemaManager.GetAllVersions()
		if err != nil {
			return errResult("failed to get all supported versions by this tool: %v", err), nil
		}
		return textResult(fmt.Sprintf("versions: %s", versions)), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

func getCollectorComponentsTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-components",
		Description: "Get all OpenTelemetry collector components",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Collector component kind: receiver, exporter, processor, connector, extension"},"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"}},"required":["kind"]}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		componentKind, ok := requireStringArg(args, "kind")
		if !ok {
			return errResult("kind argument is required"), nil
		}
		version := stringArg(args, "version", latestCollectorVersion)

		components, err := schemaManager.GetComponentNames(collectorschema.ComponentType(componentKind), version)
		if err != nil {
			return errResult("failed to get components for %s: %v", componentKind, err), nil
		}
		return textResult(fmt.Sprintf("%s", components)), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

func getCollectorReadmeTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-readme",
		Description: "Explain OpenTelemetry collector processor, receiver, exporter, extension functionality and use-cases",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Collector component kind: receiver, exporter, processor, connector, extension"},"name":{"type":"string","description":"Collector component name e.g. otlp"},"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"}},"required":["kind","name"]}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		componentKind, ok := requireStringArg(args, "kind")
		if !ok {
			return errResult("kind argument is required"), nil
		}
		componentName, ok := requireStringArg(args, "name")
		if !ok {
			return errResult("name argument is required"), nil
		}
		version := stringArg(args, "version", latestCollectorVersion)

		readme, err := schemaManager.GetComponentReadme(collectorschema.ComponentType(componentKind), componentName, version)
		if err != nil {
			return errResult("failed to get readme for %s %s: %v", componentKind, componentName, err), nil
		}
		return textResult(readme), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

func getCollectorChangelogTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-changelog",
		Description: "Returns OpenTelemetry collector changelog",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"}}}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		version := stringArg(args, "version", latestCollectorVersion)

		changelog, err := schemaManager.GetChangelog(version)
		if err != nil {
			return errResult("failed to get changelog for %s: %v", version, err), nil
		}
		return textResult(changelog), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

func getCollectorSchemaGetTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-component-schema",
		Description: "Explain OpenTelemetry collector receiver, exporter, processor, connector and extension configuration schema",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Collector component kind: receiver, exporter, processor, connector, extension"},"name":{"type":"string","description":"Collector component name e.g. otlp"},"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"}},"required":["kind","name"]}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		componentKind, ok := requireStringArg(args, "kind")
		if !ok {
			return errResult("kind argument is required"), nil
		}
		componentName, ok := requireStringArg(args, "name")
		if !ok {
			return errResult("name argument is required"), nil
		}
		version := stringArg(args, "version", latestCollectorVersion)

		schemaJSON, err := schemaManager.GetComponentSchemaJSON(collectorschema.ComponentType(componentKind), componentName, version)
		if err != nil {
			return errResult("failed to get schema for %s/%s@%s: %v", componentKind, componentName, version, err), nil
		}
		return textResult(string(schemaJSON)), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

func getCollectorSchemaValidationTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-component-schema-validation",
		Description: "Validate OpenTelemetry collector processor, receiver, exporter, extension configuration JSON",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Collector component kind: receiver, exporter, processor, connector, extension"},"name":{"type":"string","description":"Collector component name e.g. otlp"},"config":{"type":"string","description":"Collector component configuration JSON"},"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"}},"required":["kind","name","config"]}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		componentKind, ok := requireStringArg(args, "kind")
		if !ok {
			return errResult("kind argument is required"), nil
		}
		componentName, ok := requireStringArg(args, "name")
		if !ok {
			return errResult("name argument is required"), nil
		}
		config, ok := requireStringArg(args, "config")
		if !ok {
			return errResult("config argument is required"), nil
		}
		version := stringArg(args, "version", latestCollectorVersion)

		validationResult, err := schemaManager.ValidateComponentJSON(collectorschema.ComponentType(componentKind), componentName, version, []byte(config))
		if err != nil {
			return errResult("failed to validate json for %s/%s@%s: %v", componentKind, componentName, version, err), nil
		}
		return textResult(fmt.Sprintf("is valid: %v, errors: %v", validationResult.Valid(), validationResult.Errors())), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

type DeprecatedComponentFields struct {
	ComponentName    string                            `json:"componentName"`
	DeprecatedFields []collectorschema.DeprecatedField `json:"deprecatedFields"`
}

func getCollectorComponentDeprecatedTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-component-deprecated-fields",
		Description: "Return deprecated OpenTelemetry collector receiver, exporter, processor, connector and extension configuration fields",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Collector component kind: receiver, exporter, extension"},"names":{"type":"array","items":{"type":"string"},"description":"Collector component names e.g. [\"otlp\",\"jaeger\"]"},"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"}},"required":["kind","names"]}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		componentKind, ok := requireStringArg(args, "kind")
		if !ok {
			return errResult("kind argument is required"), nil
		}
		componentNames, ok := requireStringSliceArg(args, "names")
		if !ok {
			return errResult("names argument is required"), nil
		}
		version := stringArg(args, "version", latestCollectorVersion)

		var deprecations []DeprecatedComponentFields
		for _, componentName := range componentNames {
			deprecatedFields, err := schemaManager.GetDeprecatedFields(collectorschema.ComponentType(componentKind), componentName, version)
			if err != nil {
				return errResult("failed to get deprecated fields for %s/%s@%s: %v", componentKind, componentName, version, err), nil
			}
			deprecations = append(deprecations, DeprecatedComponentFields{
				ComponentName:    componentName,
				DeprecatedFields: deprecatedFields,
			})
		}
		if len(deprecations) > 0 {
			return textResult(fmt.Sprintf("deprecated fields: %+v", deprecations)), nil
		}
		return textResult("no deprecated fields found"), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

type DocumentationSearchResult struct {
	Results []collectorschema.DocumentSearchResult `json:"results"`
}

func getCollectorDocumentationRAG(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := &mcp.Tool{
		Name:        "opentelemetry-collector-rag",
		Description: "Answer questions about OpenTelemetry collector",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Query about OpenTelemetry collector documentation"},"version":{"type":"string","description":"The OpenTelemetry Collector version e.g. 0.138.0"},"kind":{"type":"string","description":"Collector component kind: receiver, exporter, processor, connector, extension"},"name":{"type":"string","description":"Collector component name e.g. otlp"}},"required":["query","version"]}`),
	}

	handler := func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArgs(req)
		query, ok := requireStringArg(args, "query")
		if !ok {
			return errResult("query argument is required"), nil
		}
		version := stringArg(args, "version", latestCollectorVersion)
		componentKind := stringArg(args, "kind", "")
		componentName := stringArg(args, "name", "")

		var result DocumentationSearchResult
		if componentKind == "" {
			results, err := schemaManager.QueryDocumentation(query, version, 3)
			if err != nil {
				return errResult("failed to process query %s: %v", query, err), nil
			}
			result = DocumentationSearchResult{Results: results}
		} else {
			results, err := schemaManager.QueryDocumentationWithFilters(query, 3, componentKind, componentName, version)
			if err != nil {
				return errResult("failed to process query %s: %v", query, err), nil
			}
			result = DocumentationSearchResult{Results: results}
		}

		return jsonResult(result), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

func boolPtr(b bool) *bool { return &b }
