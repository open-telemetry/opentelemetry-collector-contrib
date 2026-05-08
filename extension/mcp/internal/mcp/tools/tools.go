// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// tools package includes the list of tools available for the MCP extension. The contents of this
// file originated in https://github.com/pavolloffay/opentelemetry-mcp-server/blob/main/internal/tools/tools.go
package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/mcp/tools"

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pavolloffay/opentelemetry-mcp-server/modules/collectorschema"
)

// Tool represents an MCP tool with its handler
type Tool struct {
	Tool    mcp.Tool
	Handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
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

// getCollectorVersionsTool returns the collector versions tool
func getCollectorVersionsTool(schemaManager *collectorschema.SchemaManager) Tool {
	tool := mcp.NewTool("opentelemetry-collector-get-versions",
		mcp.WithDescription("Get all supported OpenTelemetry collector versions by this tool"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		versions, err := schemaManager.GetAllVersions()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get all supported versions by this tool: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("versions: %s", versions)), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

// getCollectorComponentsTool returns the collector components tool
func getCollectorComponentsTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-components",
		mcp.WithDescription("Get all OpenTelemetry collector components"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Collector component kind. It can be receiver, exporter, processor, connector and extension."),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentKind, err := request.RequireString("kind")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("kind argument is required: %v", err)), nil
		}
		version := request.GetString("version", latestCollectorVersion)

		components, err := schemaManager.GetComponentNames(collectorschema.ComponentType(componentKind), version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get components for %s: %v", componentKind, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s", components)), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

// getCollectorReadmeTool returns the collector readme tool
func getCollectorReadmeTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-readme",
		mcp.WithDescription("Explain OpenTelemetry collector processor, receiver, exporter, extension functionality and use-cases"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Collector component kind. It can be receiver, exporter, processor, connector and extension."),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Collector component name e.g. otlp"),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentKind, err := request.RequireString("kind")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("kind argument is required: %v", err)), nil
		}
		componentName, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("name argument is required: %v", err)), nil
		}
		version := request.GetString("version", latestCollectorVersion)

		readme, err := schemaManager.GetComponentReadme(collectorschema.ComponentType(componentKind), componentName, version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get readme for %s %s: %v", componentKind, componentName, err)), nil
		}
		return mcp.NewToolResultText(readme), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

// getCollectorChangelogTool returns the collector changelog tool
func getCollectorChangelogTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-changelog",
		mcp.WithDescription("Returns OpenTelemetry collector changelog"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		version := request.GetString("version", latestCollectorVersion)

		readme, err := schemaManager.GetChangelog(version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get changelog for %s: %v", version, err)), nil
		}
		return mcp.NewToolResultText(readme), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

// getCollectorSchemaGetTool returns the collector schema get tool
func getCollectorSchemaGetTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-component-schema",
		mcp.WithDescription("Explain OpenTelemetry collector receiver, exporter, processor, connector and extension configuration schema"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Collector component kind. It can be receiver, exporter, processor, connector and extension."),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Collector component name e.g. otlp"),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentKind, err := request.RequireString("kind")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("kind argument is required: %v", err)), nil
		}
		componentName, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("name argument is required: %v", err)), nil
		}
		version := request.GetString("version", latestCollectorVersion)

		schemaJSON, err := schemaManager.GetComponentSchemaJSON(collectorschema.ComponentType(componentKind), componentName, version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get schema for %s/%s@%s: %v", componentKind, componentName, version, err)), nil
		}
		return mcp.NewToolResultText(string(schemaJSON)), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

// getCollectorSchemaValidationTool returns the collector schema validation tool
func getCollectorSchemaValidationTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-component-schema-validation",
		mcp.WithDescription("Validate OpenTelemetry collector processor, receiver, exporter, extension configuration JSON"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Collector component kind. It can be receiver, exporter, processor, connector and extension."),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Collector component name e.g. otlp"),
		),
		mcp.WithString("config",
			mcp.Required(),
			mcp.Description("Collector component configuration JSON"),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentKind, err := request.RequireString("kind")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("kind argument is required: %v", err)), nil
		}
		componentName, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("name argument is required: %v", err)), nil
		}
		config, err := request.RequireString("config")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("config argument is required: %v", err)), nil
		}
		version := request.GetString("version", latestCollectorVersion)

		validationResult, err := schemaManager.ValidateComponentJSON(collectorschema.ComponentType(componentKind), componentName, version, []byte(config))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to validate json for %s/%s@%s: %v", componentKind, componentName, version, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("is valid: %v, errors: %v", validationResult.Valid(), validationResult.Errors())), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

type DeprecatedComponentFields struct {
	ComponentName    string                            `json:"componentName"`
	DeprecatedFields []collectorschema.DeprecatedField `json:"deprecatedFields"`
}

// getCollectorComponentDeprecatedTool returns the collector schema validation tool
func getCollectorComponentDeprecatedTool(schemaManager *collectorschema.SchemaManager, latestCollectorVersion string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-component-deprecated-fields",
		mcp.WithDescription("Return deprecated OpenTelemetry collector receiver, exporter, processor, connector and extension configuration fields"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
		),
		mcp.WithString("kind",
			mcp.Required(),
			mcp.Description("Collector component kind. It can be receiver, exporter, extension."),
		),
		mcp.WithArray("names",
			mcp.WithStringItems(),
			mcp.Required(),
			mcp.Description("Collector component names e.g. [\"otlp\", \"jaeger\"]"),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentKind, err := request.RequireString("kind")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("kind argument is required: %v", err)), nil
		}
		componentNames, err := request.RequireStringSlice("names")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("name argument is required: %v", err)), nil
		}
		version := request.GetString("version", latestCollectorVersion)

		var deprecations []DeprecatedComponentFields
		for _, componentName := range componentNames {
			deprecatedFields, err := schemaManager.GetDeprecatedFields(collectorschema.ComponentType(componentKind), componentName, version)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to validate json for %s/%s@%s: %v", componentKind, componentName, version, err)), nil
			}
			deprecations = append(deprecations, DeprecatedComponentFields{
				ComponentName:    componentName,
				DeprecatedFields: deprecatedFields,
			})
		}
		if len(deprecations) > 0 {
			return mcp.NewToolResultText(fmt.Sprintf("deprecated fields: %+v", deprecations)), nil
		}
		return mcp.NewToolResultText("no deprecated fields found"), nil
	}

	return Tool{Tool: tool, Handler: handler}
}

type DocumentationSearchResult struct {
	Results []collectorschema.DocumentSearchResult `json:"results"`
}

// getCollectorDocumentationRAG returns the query from the RAG
func getCollectorDocumentationRAG(schemaManager *collectorschema.SchemaManager, _ string) Tool {
	tool := mcp.NewTool("opentelemetry-collector-rag",
		mcp.WithDescription("Answer questions about OpenTelemetry collector"),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("query",
			mcp.Description("Query about OpenTelemetry collector's documentation"),
			mcp.Required(),
		),
		mcp.WithString("version",
			mcp.Description("The OpenTelemetry Collector version e.g. 0.138.0"),
			mcp.Required(),
		),
		mcp.WithString("kind",
			mcp.Description("Collector component kind. It can be receiver, exporter, processor, connector and extension. If kind is provided name has to be provided as well."),
		),
		mcp.WithString("name",
			mcp.Description("Collector component name e.g. otlp. If name is provided kind has to be provided as well."),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		undefined := "none"
		componentKind := request.GetString("kind", undefined)
		componentName := request.GetString("name", undefined)
		version, err := request.RequireString("version")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("version argument is required: %v", err)), nil
		}
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query argument is required: %v", err)), nil
		}

		if (componentKind == undefined && componentName != undefined) && (componentName == undefined && componentKind != undefined) {
			return mcp.NewToolResultError(fmt.Sprintf("if kind is provided name has to be provided as well: %v", err)), nil
		}

		var result DocumentationSearchResult
		if componentKind == undefined {
			results, err := schemaManager.QueryDocumentation(query, version, 3)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to process query %s: %v", query, err)), nil
			}
			result = DocumentationSearchResult{Results: results}
		} else {
			results, err := schemaManager.QueryDocumentationWithFilters(query, 3, componentKind, componentName, version)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to process query %s: %v", query, err)), nil
			}
			result = DocumentationSearchResult{Results: results}
		}

		return mcp.NewToolResultJSON(result)
	}

	return Tool{Tool: tool, Handler: handler}
}
