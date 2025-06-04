// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package componentchecker will define the functions and types necessary to parse component status and config components
package componentchecker // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/pipelines"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
)

const (
	receiverKind   = "receiver"
	receiversKind  = "receivers"
	processorKind  = "processor"
	processorsKind = "processors"
	exporterKind   = "exporter"
	exportersKind  = "exporters"
	extensionKind  = "extension"
	extensionsKind = "extensions"
	connectorKind  = "connector"
	connectorsKind = "connectors"
	providerKind   = "provider"
	providersKind  = "providers"
	converterKind  = "converter"
	convertersKind = "converters"
	pipelinesKind  = "pipelines"
)

func isComponentConfigured(typ component.Type, c map[component.ID]component.Config) bool {
	// Check if at least one of the component type is configured in the config map
	// Note, there could be more than one component of the same type, this function
	// only guarantees that at least one component of the type is configured
	// in the config map.
	for id := range c {
		if id.Type() == typ {
			return true
		}
	}
	return false
}

// DataToFlattenedJSONString is a helper function to ensure payload strings are
// properly formatted for JSON parsing. This is necessary due to escaped newline
// characters and whitespace causing failure on parsing when loading into
// the underlying data platform.
func DataToFlattenedJSONString(data any) string {
	replacer := strings.NewReplacer("\r", "", "\n", "")
	jsonData, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	res := replacer.Replace(string(jsonData))
	return res
}

// PopulateFullComponentsJSON creates a ModuleInfoJSON struct with all components from ModuleInfos
func PopulateFullComponentsJSON(moduleInfo service.ModuleInfos, c *confmap.Conf) (*payload.ModuleInfoJSON, error) {
	modInfo := payload.NewModuleInfoJSON()
	oc := otelcol.Config{}
	if err := c.Unmarshal(&oc); err != nil {
		return modInfo, err
	}

	for _, field := range []struct {
		data map[component.Type]service.ModuleInfo
		kind string
		cmap map[component.ID]component.Config
	}{
		{moduleInfo.Receiver, receiverKind, oc.Receivers},
		{moduleInfo.Processor, processorKind, oc.Processors},
		{moduleInfo.Exporter, exporterKind, oc.Exporters},
		{moduleInfo.Extension, extensionKind, oc.Extensions},
		{moduleInfo.Connector, connectorKind, oc.Connectors},
		// TODO: add Providers and Converters after upstream change accepted to add these to moduleinfos
	} {
		for comp, builderRef := range field.data {
			parts := strings.SplitN(builderRef.BuilderRef, " ", 2)
			enabled := isComponentConfigured(comp, field.cmap)
			modInfo.AddComponent(payload.CollectorModule{
				Type:       comp.String(),
				Kind:       field.kind,
				Gomod:      parts[0],
				Version:    parts[1],
				Configured: enabled,
			})
		}
	}
	return modInfo, nil
}

// PopulateActiveComponents gets a list of active components in the collector service
// configuration and returns a list of ServiceComponent structs for inclusion in a fleet payload
func PopulateActiveComponents(c *confmap.Conf, moduleInfoJSON *payload.ModuleInfoJSON, componentStatus map[string]any) (*[]payload.ServiceComponent, error) {
	oc := otelcol.Config{}
	var serviceComponents []payload.ServiceComponent
	if err := c.Unmarshal(&oc); err != nil {
		return &serviceComponents, err
	}

	// Extract the service configuration
	serviceConfig := oc.Service

	// Process extensions
	for _, extensionID := range serviceConfig.Extensions {
		extension := payload.ServiceComponent{
			ID:   extensionID.String(),
			Name: extensionID.Name(),
			Type: extensionID.Type().String(),
			Kind: extensionKind,
		}
		if module, ok := moduleInfoJSON.GetComponent(extension.Type, extension.Kind); ok {
			extension.Gomod = module.Gomod
			extension.Version = module.Version
		} else {
			extension.Gomod = "unknown"
			extension.Version = "unknown"
		}
		// TODO: Add component status parsing, potentially via pkg/status
		serviceComponents = append(serviceComponents, extension)
	}

	// Define a struct to generalize processing of pipeline components
	type pipelineComponent struct {
		kind       string
		kindPlural string
		components map[component.ID]component.Config
		getIDs     func(pipeline *pipelines.PipelineConfig) []component.ID
	}

	// Define the pipeline components to process
	pipelineComponents := []pipelineComponent{
		{
			kind:       receiverKind,
			kindPlural: receiversKind,
			components: oc.Receivers,
			getIDs:     func(pipeline *pipelines.PipelineConfig) []component.ID { return pipeline.Receivers },
		},
		{
			kind:       processorKind,
			kindPlural: processorsKind,
			components: oc.Processors,
			getIDs:     func(pipeline *pipelines.PipelineConfig) []component.ID { return pipeline.Processors },
		},
		{
			kind:       exporterKind,
			kindPlural: exportersKind,
			components: oc.Exporters,
			getIDs:     func(pipeline *pipelines.PipelineConfig) []component.ID { return pipeline.Exporters },
		},
		{
			kind:       connectorKind,
			kindPlural: connectorsKind,
			components: oc.Connectors,
			getIDs: func(pipeline *pipelines.PipelineConfig) []component.ID {
				// Connectors can act as both receivers and exporters, so we need to handle them separately
				return append(pipeline.Receivers, pipeline.Exporters...)
			},
		},
	}

	// Process pipelines
	for pipelineName, pipeline := range serviceConfig.Pipelines {
		for _, pc := range pipelineComponents {
			for _, componentID := range pc.getIDs(pipeline) {
				component := payload.ServiceComponent{
					ID:       componentID.String(),
					Name:     componentID.Name(),
					Type:     componentID.Type().String(),
					Kind:     pc.kind,
					Pipeline: pipelineName.String(),
				}
				if _, exists := pc.components[componentID]; !exists {
					// Skip components that are not actually part of the specified kind's components
					continue
				}
				if module, ok := moduleInfoJSON.GetComponent(component.Type, component.Kind); ok {
					component.Gomod = module.Gomod
					component.Version = module.Version
				} else {
					// This component exists but is the wrong type (e.g. a connector in the exporters pipeline)
					return nil, fmt.Errorf("component not found in Module Info, something has gone wrong with status parsing for component ID: %s", componentID.String())
				}
				// TODO: Add component status parsing, potentially via pkg/status
				serviceComponents = append(serviceComponents, component)
			}
		}
	}

	return &serviceComponents, nil
}
