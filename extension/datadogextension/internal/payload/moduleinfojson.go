// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package payload // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"

import (
	"encoding/json"
)

// ModuleInfoJSON holds data on all modules in the collector
// It is built to make checking module info quicker when building active/configured components list
// (don't need to iterate through a whole list of modules, just do key/value pair in map)
type ModuleInfoJSON struct {
	components map[string]CollectorModule
}

func NewModuleInfoJSON() *ModuleInfoJSON {
	return &ModuleInfoJSON{
		components: make(map[string]CollectorModule),
	}
}

func (m *ModuleInfoJSON) getKey(typeStr, kindStr string) string {
	return typeStr + ":" + kindStr
}

func (m *ModuleInfoJSON) AddComponent(comp CollectorModule) {
	key := m.getKey(comp.Type, comp.Kind)
	m.components[key] = comp
	// We don't ever expect two go modules to have the same type and kind
	// as collector would not be able to distinguish between them for configuration
	// and service/pipeline purposes.
}

func (m *ModuleInfoJSON) GetComponent(typeStr, kindStr string) (CollectorModule, bool) {
	key := m.getKey(typeStr, kindStr)
	comp, ok := m.components[key]
	return comp, ok
}

func (m *ModuleInfoJSON) AddComponents(components []CollectorModule) {
	for _, comp := range components {
		m.AddComponent(comp)
	}
}

func (m *ModuleInfoJSON) MarshalJSON() ([]byte, error) {
	alias := struct {
		Components []CollectorModule `json:"full_components"`
	}{
		Components: make([]CollectorModule, 0, len(m.components)),
	}
	for _, comp := range m.components {
		alias.Components = append(alias.Components, comp)
	}
	return json.Marshal(alias)
}

func (m *ModuleInfoJSON) GetFullComponentsList() []CollectorModule {
	fullComponents := make([]CollectorModule, 0, len(m.components))
	for _, comp := range m.components {
		fullComponents = append(fullComponents, comp)
	}
	return fullComponents
}
