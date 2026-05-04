// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetComponentStatusInput struct {
	Kind        string `json:"kind,omitempty" jsonschema:"Component kind (receiver, processor, exporter, connector, extension). Omit for all"`
	ComponentID string `json:"component_id,omitempty" jsonschema:"Component ID to check. Omit for all components of the kind"`
}

type ComponentStatus struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

type GetComponentStatusOutput struct {
	Components []ComponentStatus `json:"components"`
	Count      int               `json:"count"`
}

// RegisterGetComponentStatus registers the get_component_status tool
func RegisterGetComponentStatus(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetComponentStatusInput, GetComponentStatusOutput](server, &mcp.Tool{
		Name:        "get_component_status",
		Description: "Get runtime status of components. Note: Component health monitoring requires collector v0.136.0+ and is not yet fully implemented in all components.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetComponentStatusInput) (*mcp.CallToolResult, GetComponentStatusOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, GetComponentStatusOutput{}, errors.New("collector configuration not available")
		}

		components := []ComponentStatus{}

		// Get configured components
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
				for id := range sectionMap {
					// Filter by component ID if specified
					if input.ComponentID != "" && id != input.ComponentID {
						continue
					}

					// All configured components are considered "running" since the collector is running
					// Component-level health status would require collector v0.136.0+ componentstatus API
					components = append(components, ComponentStatus{
						ID:     id,
						Kind:   kind[:len(kind)-1], // Remove trailing 's'
						Status: "running",
					})
				}
			}
		}

		return nil, GetComponentStatusOutput{
			Components: components,
			Count:      len(components),
		}, nil
	})
}

type GetPipelineMetricsInput struct {
	PipelineID string `json:"pipeline_id,omitempty" jsonschema:"Pipeline ID (e.g. 'traces', 'metrics/prod'). Omit for all pipelines"`
}

type PipelineMetrics struct {
	PipelineID     string `json:"pipeline_id"`
	ReceiverCount  int    `json:"receiver_count"`
	ProcessorCount int    `json:"processor_count"`
	ExporterCount  int    `json:"exporter_count"`
	Status         string `json:"status"`
}

type GetPipelineMetricsOutput struct {
	Pipelines []PipelineMetrics `json:"pipelines"`
	Count     int               `json:"count"`
}

// RegisterGetPipelineMetrics registers the get_pipeline_metrics tool
func RegisterGetPipelineMetrics(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetPipelineMetricsInput, GetPipelineMetricsOutput](server, &mcp.Tool{
		Name:        "get_pipeline_metrics",
		Description: "Get pipeline configuration metrics. Returns component counts and pipeline status. For detailed telemetry metrics, use get_recent_metrics.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetPipelineMetricsInput) (*mcp.CallToolResult, GetPipelineMetricsOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, GetPipelineMetricsOutput{}, errors.New("collector configuration not available")
		}

		pipelines := []PipelineMetrics{}

		pipelinesConf := conf.Get("service::pipelines")
		if pipelinesConf == nil {
			return nil, GetPipelineMetricsOutput{
				Pipelines: pipelines,
				Count:     0,
			}, nil
		}

		if pipelinesMap, ok := pipelinesConf.(map[string]any); ok {
			for pipelineID, pipelineConfig := range pipelinesMap {
				// Filter by pipeline ID if specified
				if input.PipelineID != "" && pipelineID != input.PipelineID {
					continue
				}

				metrics := PipelineMetrics{
					PipelineID: pipelineID,
					Status:     "running", // If collector is running, pipeline is running
				}

				if pipelineMap, ok := pipelineConfig.(map[string]any); ok {
					// Count receivers
					if receivers, ok := pipelineMap["receivers"].([]any); ok {
						metrics.ReceiverCount = len(receivers)
					}
					// Count processors
					if processors, ok := pipelineMap["processors"].([]any); ok {
						metrics.ProcessorCount = len(processors)
					}
					// Count exporters
					if exporters, ok := pipelineMap["exporters"].([]any); ok {
						metrics.ExporterCount = len(exporters)
					}
				}

				pipelines = append(pipelines, metrics)
			}
		}

		return nil, GetPipelineMetricsOutput{
			Pipelines: pipelines,
			Count:     len(pipelines),
		}, nil
	})
}

type GetExtensionsOutput struct {
	Count      int      `json:"count"`
	Extensions []string `json:"extensions"`
}

// RegisterGetExtensions registers the get_extensions tool
func RegisterGetExtensions(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_extensions",
		Description: "List all running extensions",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input any) (*mcp.CallToolResult, GetExtensionsOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		host := ext.GetHost()
		if host == nil {
			return nil, GetExtensionsOutput{}, errors.New("component host not yet available")
		}

		extensions := host.GetExtensions()
		result := make([]string, 0, len(extensions))
		for id := range extensions {
			result = append(result, id.String())
		}

		return nil, GetExtensionsOutput{
			Count:      len(result),
			Extensions: result,
		}, nil
	})
}
