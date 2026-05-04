// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NOTE: Configuration validation is read-only in this implementation.
// The MCP extension can inspect and validate configuration but cannot
// persist changes to disk or trigger collector reloads. This would require:
// - File system write access to the config file
// - Ability to trigger collector reload/restart
// - Proper validation of the entire configuration graph
//
// These tools provide configuration validation capabilities only.

type ValidateConfigSectionInput struct {
	Section string         `json:"section" jsonschema:"Configuration section to validate (receivers, processors, exporters, etc.),required"`
	Config  map[string]any `json:"config" jsonschema:"Configuration for the section to validate,required"`
}

type ValidateConfigSectionOutput struct {
	Valid      bool     `json:"valid"`
	Message    string   `json:"message"`
	Validation []string `json:"validation,omitempty"`
}

// RegisterValidateConfigSection registers the validate_config_section tool
func RegisterValidateConfigSection(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[ValidateConfigSectionInput, ValidateConfigSectionOutput](server, &mcp.Tool{
		Name:        "validate_config_section",
		Description: "Validate a configuration section structure (receivers, processors, exporters, etc.)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ValidateConfigSectionInput) (*mcp.CallToolResult, ValidateConfigSectionOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, ValidateConfigSectionOutput{}, NewConfigError("validate_config_section", "", ErrConfigNotAvailable)
		}

		// Validate the section exists in valid config sections
		validSections := map[string]bool{
			"receivers":  true,
			"processors": true,
			"exporters":  true,
			"connectors": true,
			"extensions": true,
			"service":    true,
		}

		if !validSections[input.Section] {
			return nil, ValidateConfigSectionOutput{}, NewConfigError("validate_config_section", input.Section, ErrSectionNotFound)
		}

		// Validate basic structure
		validationIssues := []string{}

		if input.Config == nil {
			validationIssues = append(validationIssues, "config cannot be nil")
		}

		// For component sections, validate each component has valid structure
		if input.Section != "service" {
			for id, cfg := range input.Config {
				if cfg == nil {
					validationIssues = append(validationIssues, fmt.Sprintf("component %s has nil config", id))
				}
			}
		}

		if len(validationIssues) > 0 {
			return nil, ValidateConfigSectionOutput{
				Valid:      false,
				Message:    "Configuration validation failed",
				Validation: validationIssues,
			}, fmt.Errorf("configuration validation failed: %v", validationIssues)
		}

		return nil, ValidateConfigSectionOutput{
			Valid:      true,
			Message:    "Configuration structure is valid. Note: Changes are not persisted - this is a read-only validation.",
			Validation: []string{"Configuration structure validated successfully"},
		}, nil
	})
}

type AddComponentInput struct {
	Kind        string         `json:"kind" jsonschema:"Component kind (receiver, processor, exporter, connector, extension),required"`
	ComponentID string         `json:"component_id" jsonschema:"Component ID (e.g. 'otlp', 'batch', 'debug'),required"`
	Config      map[string]any `json:"config" jsonschema:"Component configuration,required"`
}

type AddComponentOutput struct {
	Message    string   `json:"message"`
	Validation []string `json:"validation,omitempty"`
}

// RegisterAddComponent registers the add_component tool
func RegisterAddComponent(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[AddComponentInput, AddComponentOutput](server, &mcp.Tool{
		Name:        "add_component",
		Description: "Validate adding a new component to configuration (read-only)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input AddComponentInput) (*mcp.CallToolResult, AddComponentOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, AddComponentOutput{}, NewConfigError("add_component", "", ErrConfigNotAvailable)
		}

		// Validate kind
		if _, err := parseComponentKind(input.Kind); err != nil {
			return nil, AddComponentOutput{}, err
		}

		section, ok := validKindsMap()[input.Kind]
		if !ok {
			return nil, AddComponentOutput{}, fmt.Errorf("invalid component kind: %s", input.Kind)
		}

		// Check if component already exists
		existingConfig := conf.Get(section + "::" + input.ComponentID)
		if existingConfig != nil {
			return nil, AddComponentOutput{
				Message:    fmt.Sprintf("Component %s/%s already exists", input.Kind, input.ComponentID),
				Validation: []string{fmt.Sprintf("Component %s already exists in %s", input.ComponentID, section)},
			}, errors.New("component already exists")
		}

		// Validate config is not nil
		if input.Config == nil {
			return nil, AddComponentOutput{
				Message:    "Invalid component configuration",
				Validation: []string{"Component config cannot be nil"},
			}, errors.New("component config cannot be nil")
		}

		return nil, AddComponentOutput{
			Message:    fmt.Sprintf("Component %s/%s configuration is valid. Note: Changes are not persisted - this is a read-only validation.", input.Kind, input.ComponentID),
			Validation: []string{"Component structure validated successfully"},
		}, nil
	})
}

type RemoveComponentInput struct {
	Kind        string `json:"kind" jsonschema:"Component kind (receiver, processor, exporter, connector, extension),required"`
	ComponentID string `json:"component_id" jsonschema:"Component ID to remove,required"`
}

type RemoveComponentOutput struct {
	Message           string   `json:"message"`
	Validation        []string `json:"validation,omitempty"`
	AffectedPipelines []string `json:"affected_pipelines,omitempty"`
}

// RegisterRemoveComponent registers the remove_component tool
func RegisterRemoveComponent(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[RemoveComponentInput, RemoveComponentOutput](server, &mcp.Tool{
		Name:        "remove_component",
		Description: "Validate removing a component from configuration (read-only)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RemoveComponentInput) (*mcp.CallToolResult, RemoveComponentOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, RemoveComponentOutput{}, NewConfigError("remove_component", "", ErrConfigNotAvailable)
		}

		// Validate kind
		if _, err := parseComponentKind(input.Kind); err != nil {
			return nil, RemoveComponentOutput{}, err
		}

		section, ok := validKindsMap()[input.Kind]
		if !ok {
			return nil, RemoveComponentOutput{}, fmt.Errorf("invalid component kind: %s", input.Kind)
		}

		// Check if component exists
		existingConfig := conf.Get(section + "::" + input.ComponentID)
		if existingConfig == nil {
			return nil, RemoveComponentOutput{}, NewConfigError("remove_component", input.ComponentID, ErrComponentNotFound)
		}

		affectedPipelines := []string{}
		validation := []string{}

		// For extensions, check service::extensions
		if input.Kind == "extension" {
			serviceExtensions := conf.Get("service::extensions")
			if serviceExtensions != nil {
				if extList, ok := serviceExtensions.([]any); ok {
					for _, item := range extList {
						if itemStr, ok := item.(string); ok && itemStr == input.ComponentID {
							validation = append(validation, fmt.Sprintf("Warning: Extension %s is referenced in service.extensions", input.ComponentID))
							break
						}
					}
				}
			}
		}

		// Check which pipelines use this component
		pipelinesConf := conf.Get("service::pipelines")
		if pipelinesConf != nil {
			if pipelines, ok := pipelinesConf.(map[string]any); ok {
				for pipelineID, pipelineConfig := range pipelines {
					if pipelineMap, ok := pipelineConfig.(map[string]any); ok {
						// Check receivers, processors, exporters
						for _, listKey := range []string{section} {
							if list, ok := pipelineMap[listKey].([]any); ok {
								for _, item := range list {
									if itemStr, ok := item.(string); ok && itemStr == input.ComponentID {
										affectedPipelines = append(affectedPipelines, pipelineID)
										break
									}
								}
							}
						}
					}
				}
			}
		}

		if len(validation) == 0 {
			validation = append(validation, fmt.Sprintf("Component %s/%s exists and can be removed", input.Kind, input.ComponentID))
		}
		if len(affectedPipelines) > 0 {
			validation = append(validation, fmt.Sprintf("Warning: Component is used in %d pipeline(s)", len(affectedPipelines)))
		}

		return nil, RemoveComponentOutput{
			Message:           fmt.Sprintf("Component %s/%s removal validated. Note: Changes are not persisted - this is a read-only validation.", input.Kind, input.ComponentID),
			Validation:        validation,
			AffectedPipelines: affectedPipelines,
		}, nil
	})
}

type ValidateConfigInput struct {
	Config map[string]any `json:"config" jsonschema:"Complete configuration to validate,required"`
}

type ValidateConfigOutput struct {
	Valid      bool     `json:"valid"`
	Message    string   `json:"message"`
	Validation []string `json:"validation,omitempty"`
}

// RegisterValidateConfig registers the validate_config tool
func RegisterValidateConfig(server *mcp.Server, _ ExtensionContext) {
	mcp.AddTool[ValidateConfigInput, ValidateConfigOutput](server, &mcp.Tool{
		Name:        "validate_config",
		Description: "Validate a proposed configuration structure",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ValidateConfigInput) (*mcp.CallToolResult, ValidateConfigOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		if input.Config == nil {
			return nil, ValidateConfigOutput{
				Valid:      false,
				Message:    "Configuration is nil",
				Validation: []string{"Config cannot be nil"},
			}, errors.New("config cannot be nil")
		}

		validationIssues := []string{}

		// Check for required top-level sections
		requiredSections := []string{"receivers", "exporters", "service"}
		for _, section := range requiredSections {
			if input.Config[section] == nil {
				validationIssues = append(validationIssues, fmt.Sprintf("Missing required section: %s", section))
			}
		}

		// Validate service section has pipelines
		if service, ok := input.Config["service"].(map[string]any); ok {
			if service["pipelines"] == nil {
				validationIssues = append(validationIssues, "Service section must have pipelines")
			}
		}

		if len(validationIssues) > 0 {
			return nil, ValidateConfigOutput{
				Valid:      false,
				Message:    "Configuration validation failed",
				Validation: validationIssues,
			}, nil
		}

		return nil, ValidateConfigOutput{
			Valid:      true,
			Message:    "Configuration structure is valid",
			Validation: []string{"All required sections present", "Service pipelines defined"},
		}, nil
	})
}

type UpdatePipelineInput struct {
	PipelineID string         `json:"pipeline_id" jsonschema:"Pipeline ID (e.g. 'traces', 'metrics/prod'),required"`
	Config     map[string]any `json:"config" jsonschema:"Pipeline configuration with receivers, processors, exporters,required"`
}

type UpdatePipelineOutput struct {
	Message    string   `json:"message"`
	Validation []string `json:"validation,omitempty"`
}

// RegisterUpdatePipeline registers the update_pipeline tool
func RegisterUpdatePipeline(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[UpdatePipelineInput, UpdatePipelineOutput](server, &mcp.Tool{
		Name:        "update_pipeline",
		Description: "Validate modifying a pipeline configuration (read-only)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input UpdatePipelineInput) (*mcp.CallToolResult, UpdatePipelineOutput, error) { //nolint:revive // ctx unused but kept for interface compatibility
		conf := ext.GetCollectorConf()
		if conf == nil {
			return nil, UpdatePipelineOutput{}, NewConfigError("update_pipeline", "", ErrConfigNotAvailable)
		}

		// Validate pipeline config structure
		validationIssues := []string{}

		if input.Config == nil {
			validationIssues = append(validationIssues, "Pipeline config cannot be nil")
		} else {
			// All pipelines should have receivers and exporters
			if input.Config["receivers"] == nil {
				validationIssues = append(validationIssues, "Pipeline must have receivers")
			}
			if input.Config["exporters"] == nil {
				validationIssues = append(validationIssues, "Pipeline must have exporters")
			}

			// Validate receivers/processors/exporters are lists
			for _, field := range []string{"receivers", "processors", "exporters"} {
				if input.Config[field] != nil {
					if _, ok := input.Config[field].([]any); !ok {
						validationIssues = append(validationIssues, fmt.Sprintf("%s must be a list", field))
					}
				}
			}
		}

		if len(validationIssues) > 0 {
			return nil, UpdatePipelineOutput{
				Message:    "Pipeline validation failed",
				Validation: validationIssues,
			}, fmt.Errorf("pipeline validation failed: %v", validationIssues)
		}

		return nil, UpdatePipelineOutput{
			Message:    fmt.Sprintf("Pipeline %s configuration is valid. Note: Changes are not persisted - this is a read-only validation.", input.PipelineID),
			Validation: []string{"Pipeline structure validated successfully"},
		}, nil
	})
}
