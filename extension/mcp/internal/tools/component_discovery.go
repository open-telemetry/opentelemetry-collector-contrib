// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"
)

type ListAvailableComponentsInput struct {
	Kind string `json:"kind" jsonschema:"Filter by component kind (receiver, processor, exporter, connector, extension). Omit for all,omitempty"`
}

type ComponentInfo struct {
	Type    string `json:"type"`
	Version string `json:"version"`
}

type ListAvailableComponentsOutput struct {
	Receivers  []ComponentInfo `json:"receivers,omitempty"`
	Processors []ComponentInfo `json:"processors,omitempty"`
	Exporters  []ComponentInfo `json:"exporters,omitempty"`
	Connectors []ComponentInfo `json:"connectors,omitempty"`
	Extensions []ComponentInfo `json:"extensions,omitempty"`
}

// RegisterListAvailableComponents registers the list_available_components tool
func RegisterListAvailableComponents(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[ListAvailableComponentsInput, ListAvailableComponentsOutput](server, &mcp.Tool{
		Name:        "list_available_components",
		Description: "List all component types available in this collector build with their versions",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListAvailableComponentsInput) (*mcp.CallToolResult, ListAvailableComponentsOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		moduleInfos := ext.GetModuleInfos()
		if moduleInfos == nil {
			return nil, ListAvailableComponentsOutput{}, errors.New("host does not provide ModuleInfo capability - component discovery not available")
		}

		output := ListAvailableComponentsOutput{}

		// Helper to convert module info map to sorted ComponentInfo slice
		toComponentInfos := func(infos map[component.Type]service.ModuleInfo) []ComponentInfo {
			result := make([]ComponentInfo, 0, len(infos))
			for compType, info := range infos {
				result = append(result, ComponentInfo{
					Type:    compType.String(),
					Version: info.BuilderRef,
				})
			}
			sort.Slice(result, func(i, j int) bool {
				return result[i].Type < result[j].Type
			})
			return result
		}

		// Filter by kind if requested
		if input.Kind == "" || input.Kind == "receiver" {
			output.Receivers = toComponentInfos(moduleInfos.Receiver)
		}
		if input.Kind == "" || input.Kind == "processor" {
			output.Processors = toComponentInfos(moduleInfos.Processor)
		}
		if input.Kind == "" || input.Kind == "exporter" {
			output.Exporters = toComponentInfos(moduleInfos.Exporter)
		}
		if input.Kind == "" || input.Kind == "connector" {
			output.Connectors = toComponentInfos(moduleInfos.Connector)
		}
		if input.Kind == "" || input.Kind == "extension" {
			output.Extensions = toComponentInfos(moduleInfos.Extension)
		}

		return nil, output, nil
	})
}

type GetComponentSchemaInput struct {
	Kind          string `json:"kind" jsonschema:"Component kind (receiver, processor, exporter, connector, extension),required"`
	ComponentType string `json:"component_type" jsonschema:"Component type (e.g. 'otlp', 'batch', 'debug'),required"`
}

type GetComponentSchemaOutput struct {
	ComponentType string         `json:"component_type"`
	Kind          string         `json:"kind"`
	ConfigType    string         `json:"config_type"`
	Schema        map[string]any `json:"schema"`
}

// marshalConfigSchema uses confmap to properly encode a config object with all defaults and tag handling
func marshalConfigSchema(cfg component.Config) (map[string]any, error) {
	if cfg == nil {
		return make(map[string]any), nil
	}

	// Use collector's own encoding logic
	conf := confmap.New()
	if err := conf.Marshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Returns properly structured map with defaults
	return conf.ToStringMap(), nil
}

// RegisterGetComponentSchema registers the get_component_schema tool
func RegisterGetComponentSchema(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetComponentSchemaInput, GetComponentSchemaOutput](server, &mcp.Tool{
		Name:        "get_component_schema",
		Description: "Get component configuration schema by reflecting on the default config structure",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetComponentSchemaInput) (*mcp.CallToolResult, GetComponentSchemaOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		// Validate kind
		compKind, err := parseComponentKind(input.Kind)
		if err != nil {
			return nil, GetComponentSchemaOutput{}, err
		}

		// Parse component type
		compType, err := component.NewType(input.ComponentType)
		if err != nil {
			return nil, GetComponentSchemaOutput{}, fmt.Errorf("invalid component type: %w", err)
		}

		// Get factory
		componentFactory := ext.GetComponentFactory()
		if componentFactory == nil {
			return nil, GetComponentSchemaOutput{}, errors.New("host does not provide ComponentFactory capability - cannot retrieve factory")
		}

		factory := componentFactory.GetFactory(compKind, compType)
		if factory == nil {
			return nil, GetComponentSchemaOutput{}, fmt.Errorf("factory not found for %s/%s", input.Kind, input.ComponentType)
		}

		// Get default config
		defaultCfg := factory.CreateDefaultConfig()
		if defaultCfg == nil {
			return nil, GetComponentSchemaOutput{}, errors.New("factory returned nil default config")
		}

		// Marshal config to map using collector's encoding logic
		schema, err := marshalConfigSchema(defaultCfg)
		if err != nil {
			return nil, GetComponentSchemaOutput{}, fmt.Errorf("failed to marshal config schema: %w", err)
		}

		// Get the config type name
		cfgType := reflect.TypeOf(defaultCfg)
		configTypeName := cfgType.String()

		return nil, GetComponentSchemaOutput{
			ComponentType: input.ComponentType,
			Kind:          input.Kind,
			ConfigType:    configTypeName,
			Schema:        schema,
		}, nil
	})
}

type GetFactoryInfoInput struct {
	Kind          string `json:"kind" jsonschema:"Component kind (receiver, processor, exporter, connector, extension),required"`
	ComponentType string `json:"component_type" jsonschema:"Component type (e.g. 'otlp', 'batch', 'debug'),required"`
}

type GetFactoryInfoOutput struct {
	Type           string `json:"type"`
	Kind           string `json:"kind"`
	StabilityLevel string `json:"stability_level"`
	Version        string `json:"version"`
	Available      bool   `json:"available"`
}

// RegisterGetFactoryInfo registers the get_factory_info tool
func RegisterGetFactoryInfo(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetFactoryInfoInput, GetFactoryInfoOutput](server, &mcp.Tool{
		Name:        "get_factory_info",
		Description: "Get factory metadata for a specific component type including stability level",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetFactoryInfoInput) (*mcp.CallToolResult, GetFactoryInfoOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		// Validate kind
		compKind, err := parseComponentKind(input.Kind)
		if err != nil {
			return nil, GetFactoryInfoOutput{}, err
		}

		// Parse component type
		compType, err := component.NewType(input.ComponentType)
		if err != nil {
			return nil, GetFactoryInfoOutput{}, fmt.Errorf("invalid component type: %w", err)
		}

		// Get version from module infos
		version := "unknown"
		moduleInfos := ext.GetModuleInfos()
		if moduleInfos != nil {
			var moduleInfo service.ModuleInfo
			var found bool
			switch compKind {
			case component.KindReceiver:
				moduleInfo, found = moduleInfos.Receiver[compType]
			case component.KindProcessor:
				moduleInfo, found = moduleInfos.Processor[compType]
			case component.KindExporter:
				moduleInfo, found = moduleInfos.Exporter[compType]
			case component.KindConnector:
				moduleInfo, found = moduleInfos.Connector[compType]
			case component.KindExtension:
				moduleInfo, found = moduleInfos.Extension[compType]
			}

			if found {
				version = moduleInfo.BuilderRef
			}
		}

		// Try to get factory
		componentFactory := ext.GetComponentFactory()
		if componentFactory == nil {
			return nil, GetFactoryInfoOutput{
				Type:      input.ComponentType,
				Kind:      input.Kind,
				Version:   version,
				Available: moduleInfos != nil && version != "unknown",
			}, nil
		}

		factory := componentFactory.GetFactory(compKind, compType)
		if factory == nil {
			return nil, GetFactoryInfoOutput{
				Type:      input.ComponentType,
				Kind:      input.Kind,
				Version:   version,
				Available: false,
			}, nil
		}

		// Get stability level
		stabilityLevel := "unknown"
		if factory.Type() == compType {
			// Factory exists, try to get stability via type assertion
			// Different factory types have different stability methods
			switch compKind {
			case component.KindReceiver:
				if rf, ok := factory.(interface {
					ReceiverStability() component.StabilityLevel
				}); ok {
					stabilityLevel = rf.ReceiverStability().String()
				}
			case component.KindProcessor:
				if pf, ok := factory.(interface {
					ProcessorStability() component.StabilityLevel
				}); ok {
					stabilityLevel = pf.ProcessorStability().String()
				}
			case component.KindExporter:
				if ef, ok := factory.(interface {
					ExporterStability() component.StabilityLevel
				}); ok {
					stabilityLevel = ef.ExporterStability().String()
				}
			case component.KindConnector:
				if cf, ok := factory.(interface {
					ConnectorStability() component.StabilityLevel
				}); ok {
					stabilityLevel = cf.ConnectorStability().String()
				}
			case component.KindExtension:
				if ef, ok := factory.(interface {
					ExtensionStability() component.StabilityLevel
				}); ok {
					stabilityLevel = ef.ExtensionStability().String()
				}
			}
		}

		return nil, GetFactoryInfoOutput{
			Type:           input.ComponentType,
			Kind:           input.Kind,
			StabilityLevel: stabilityLevel,
			Version:        version,
			Available:      true,
		}, nil
	})
}
