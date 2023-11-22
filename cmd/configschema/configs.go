// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

const (
	receiver  = "receiver"
	extension = "extension"
	processor = "processor"
	exporter  = "exporter"
	connector = "connector"
)

// CfgInfo contains a component config instance, as well as its group name and
// type.
type CfgInfo struct {
	// the name of the component group, e.g. "receiver"
	Group string
	// the component type, e.g. "otlpreceiver.Config"
	Type component.Type
	// an instance of the component's configuration struct
	CfgInstance any
}

// GetAllCfgInfos accepts a Factories struct, then creates and returns a CfgInfo
// for each of its components.
func GetAllCfgInfos(components otelcol.Factories) []CfgInfo {
	out := make([]CfgInfo, len(components.Receivers)+len(components.Extensions)+len(components.Processors)+len(components.Exporters)+len(components.Connectors))
	i := 0
	for _, f := range components.Receivers {
		out[i] = CfgInfo{
			Type:        f.Type(),
			Group:       receiver,
			CfgInstance: f.CreateDefaultConfig(),
		}
		i++
	}
	for _, f := range components.Extensions {
		out[i] = CfgInfo{
			Type:        f.Type(),
			Group:       extension,
			CfgInstance: f.CreateDefaultConfig(),
		}
		i++
	}
	for _, f := range components.Processors {
		out[i] = CfgInfo{
			Type:        f.Type(),
			Group:       processor,
			CfgInstance: f.CreateDefaultConfig(),
		}
		i++
	}
	for _, f := range components.Exporters {
		out[i] = CfgInfo{
			Type:        f.Type(),
			Group:       exporter,
			CfgInstance: f.CreateDefaultConfig(),
		}
		i++
	}
	for _, f := range components.Connectors {
		out[i] = CfgInfo{
			Type:        f.Type(),
			Group:       connector,
			CfgInstance: f.CreateDefaultConfig(),
		}
		i++
	}
	return out
}

// GetCfgInfo accepts a Factories struct, then creates and returns the default
// config for the component specified by the passed-in componentType and
// componentName.
func GetCfgInfo(components otelcol.Factories, componentType, componentName string) (CfgInfo, error) {
	t := component.Type(componentName)
	switch componentType {
	case receiver:
		f := components.Receivers[t]
		if f == nil {
			return CfgInfo{}, fmt.Errorf("unknown %s name %q", componentType, componentName)
		}
		return CfgInfo{
			Type:        f.Type(),
			Group:       componentType,
			CfgInstance: f.CreateDefaultConfig(),
		}, nil
	case processor:
		f := components.Processors[t]
		if f == nil {
			return CfgInfo{}, fmt.Errorf("unknown %s name %q", componentType, componentName)
		}
		return CfgInfo{
			Type:        f.Type(),
			Group:       componentType,
			CfgInstance: f.CreateDefaultConfig(),
		}, nil
	case exporter:
		f := components.Exporters[t]
		if f == nil {
			return CfgInfo{}, fmt.Errorf("unknown %s name %q", componentType, componentName)
		}
		return CfgInfo{
			Type:        f.Type(),
			Group:       componentType,
			CfgInstance: f.CreateDefaultConfig(),
		}, nil
	case connector:
		f := components.Connectors[t]
		if f == nil {
			return CfgInfo{}, fmt.Errorf("unknown %s name %q", componentType, componentName)
		}
		return CfgInfo{
			Type:        f.Type(),
			Group:       componentType,
			CfgInstance: f.CreateDefaultConfig(),
		}, nil
	case extension:
		f := components.Extensions[t]
		if f == nil {
			return CfgInfo{}, fmt.Errorf("unknown %s name %q", componentType, componentName)
		}
		return CfgInfo{
			Type:        f.Type(),
			Group:       componentType,
			CfgInstance: f.CreateDefaultConfig(),
		}, nil
	}
	return CfgInfo{}, fmt.Errorf("unknown component type %q", componentType)
}
