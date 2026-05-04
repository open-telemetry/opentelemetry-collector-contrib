// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetConfigInput struct {
	Section string `json:"section,omitempty" jsonschema:"Configuration section to retrieve (receivers processors exporters connectors extensions service telemetry). Omit for full config"`
}

// RegisterGetConfig registers the get_config tool
func RegisterGetConfig(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetConfigInput, any](server, &mcp.Tool{
		Name:        "get_config",
		Description: "Get the current collector configuration. Returns JSON with ALL defaults expanded (including zero values, empty strings, etc.). When writing configs, omit fields set to their default values to keep YAML concise. Time durations are in nanoseconds (e.g., 30000000000 = 30s).",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetConfigInput) (*mcp.CallToolResult, any, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, nil, NewConfigError("get_config", "", ErrConfigNotAvailable)
		}

		if input.Section == "" {
			// Return full config
			return nil, conf.ToStringMap(), nil
		}

		// Return specific section
		result := conf.Get(input.Section)
		if result == nil {
			return nil, nil, NewConfigError("get_config", input.Section, ErrSectionNotFound)
		}
		return nil, result, nil
	})
}

type GetComponentConfigInput struct {
	ComponentID string `json:"component_id" jsonschema:"Component ID (e.g. 'otlp' 'otlp/custom' 'batch'),required"`
	Kind        string `json:"kind" jsonschema:"Component kind (receiver processor exporter connector extension),required"`
}

// RegisterGetComponentConfig registers the get_component_config tool
func RegisterGetComponentConfig(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetComponentConfigInput, any](server, &mcp.Tool{
		Name:        "get_component_config",
		Description: "Get configuration for a specific component instance. Returns the effective configuration as reported by the collector.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetComponentConfigInput) (*mcp.CallToolResult, any, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, nil, NewConfigError("get_component_config", "", ErrConfigNotAvailable)
		}

		section := input.Kind + "s"
		subConf, err := conf.Sub(section + "::" + input.ComponentID)
		if err != nil {
			return nil, nil, NewConfigError("get_component_config", input.ComponentID, ErrComponentNotFound)
		}
		if subConf == nil {
			return nil, nil, NewConfigError("get_component_config", input.ComponentID, ErrComponentNotFound)
		}

		return nil, subConf.ToStringMap(), nil
	})
}

type ListConfiguredComponentsInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"Filter by component kind (receiver processor exporter connector extension). Omit for all"`
}

type ListConfiguredComponentsOutput struct {
	Components map[string][]string `json:"components"`
}

// RegisterListConfiguredComponents registers the list_configured_components tool
func RegisterListConfiguredComponents(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[ListConfiguredComponentsInput, ListConfiguredComponentsOutput](server, &mcp.Tool{
		Name:        "list_configured_components",
		Description: "List all currently configured components",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListConfiguredComponentsInput) (*mcp.CallToolResult, ListConfiguredComponentsOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, ListConfiguredComponentsOutput{}, NewConfigError("list_configured_components", "", ErrConfigNotAvailable)
		}

		result := make(map[string][]string)

		kinds := []string{"receivers", "processors", "exporters", "connectors", "extensions"}
		if input.Kind != "" {
			kinds = []string{input.Kind + "s"}
		}

		for _, kind := range kinds {
			section := conf.Get(kind)
			if section == nil {
				continue
			}

			if sectionMap, ok := section.(map[string]any); ok {
				components := make([]string, 0, len(sectionMap))
				for id := range sectionMap {
					components = append(components, id)
				}
				result[strings.TrimSuffix(kind, "s")] = components
			}
		}

		return nil, ListConfiguredComponentsOutput{Components: result}, nil
	})
}

type GetPipelineConfigInput struct {
	PipelineID string `json:"pipeline_id" jsonschema:"Pipeline ID (e.g. 'traces' 'metrics/prod'),required"`
}

// RegisterGetPipelineConfig registers the get_pipeline_config tool
func RegisterGetPipelineConfig(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetPipelineConfigInput, any](server, &mcp.Tool{
		Name:        "get_pipeline_config",
		Description: "Get configuration for a specific pipeline",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetPipelineConfigInput) (*mcp.CallToolResult, any, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, nil, NewConfigError("get_pipeline_config", "", ErrConfigNotAvailable)
		}

		pipelineConfig := conf.Get("service::pipelines::" + input.PipelineID)
		if pipelineConfig == nil {
			return nil, nil, NewConfigError("get_pipeline_config", input.PipelineID, ErrPipelineNotFound)
		}

		return nil, pipelineConfig, nil
	})
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
