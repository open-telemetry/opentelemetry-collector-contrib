// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package payload // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewModuleInfoJSON(t *testing.T) {
	modInfo := NewModuleInfoJSON()
	assert.NotNil(t, modInfo)
	assert.NotNil(t, modInfo.components)
	assert.Empty(t, modInfo.components)
}

func TestAddComponent(t *testing.T) {
	modInfo := NewModuleInfoJSON()
	comp := CollectorModule{
		Type:       "receiver",
		Kind:       "otlp",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpreceiver",
		Version:    "v0.30.0",
		Configured: true,
	}
	modInfo.AddComponent(comp)
	assert.Len(t, modInfo.components, 1)
	key := modInfo.getKey(comp.Type, comp.Kind)
	assert.Equal(t, comp, modInfo.components[key])
}

func TestGetComponent(t *testing.T) {
	modInfo := NewModuleInfoJSON()
	comp := CollectorModule{
		Type:       "receiver",
		Kind:       "otlp",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpreceiver",
		Version:    "v0.30.0",
		Configured: true,
	}
	modInfo.AddComponent(comp)
	retrievedComp, ok := modInfo.GetComponent("receiver", "otlp")
	assert.True(t, ok)
	assert.Equal(t, comp, retrievedComp)

	_, ok = modInfo.GetComponent("processor", "batch")
	assert.False(t, ok)
}

func TestModuleInfoJSON_MarshalJSON(t *testing.T) {
	modInfo := NewModuleInfoJSON()
	comp1 := CollectorModule{
		Type:       "receiver",
		Kind:       "otlp",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpreceiver",
		Version:    "v0.30.0",
		Configured: true,
	}
	comp2 := CollectorModule{
		Type:       "processor",
		Kind:       "batch",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchprocessor",
		Version:    "v0.30.0",
		Configured: true,
	}
	modInfo.AddComponent(comp1)
	modInfo.AddComponent(comp2)

	jsonData, err := json.Marshal(modInfo)
	assert.NoError(t, err)

	var actualJSON map[string]any
	err = json.Unmarshal(jsonData, &actualJSON)
	assert.NoError(t, err)

	expectedJSON := `{
			"full_components": [
					{
							"type": "receiver",
							"kind": "otlp",
							"gomod": "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpreceiver",
							"version": "v0.30.0",
							"configured": true
					},
					{
							"type": "processor",
							"kind": "batch",
							"gomod": "github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchprocessor",
							"version": "v0.30.0",
							"configured": true
					}
			]
	}`
	var expectedJSONMap map[string]any
	err = json.Unmarshal([]byte(expectedJSON), &expectedJSONMap)
	assert.NoError(t, err)

	assert.ElementsMatch(t, expectedJSONMap["full_components"], actualJSON["full_components"])
}

func TestAddComponents(t *testing.T) {
	modInfo := NewModuleInfoJSON()
	comp1 := CollectorModule{
		Type:       "receiver",
		Kind:       "otlp",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpreceiver",
		Version:    "v0.30.0",
		Configured: true,
	}
	comp2 := CollectorModule{
		Type:       "processor",
		Kind:       "batch",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchprocessor",
		Version:    "v0.30.0",
		Configured: true,
	}
	modInfo.AddComponents([]CollectorModule{comp1, comp2})

	assert.Len(t, modInfo.components, 2)
	key1 := modInfo.getKey(comp1.Type, comp1.Kind)
	key2 := modInfo.getKey(comp2.Type, comp2.Kind)
	assert.Equal(t, comp1, modInfo.components[key1])
	assert.Equal(t, comp2, modInfo.components[key2])
}

func TestGetFullComponentsList(t *testing.T) {
	modInfo := NewModuleInfoJSON()
	comp1 := CollectorModule{
		Type:       "receiver",
		Kind:       "otlp",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpreceiver",
		Version:    "v0.30.0",
		Configured: true,
	}
	comp2 := CollectorModule{
		Type:       "processor",
		Kind:       "batch",
		Gomod:      "github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchprocessor",
		Version:    "v0.30.0",
		Configured: true,
	}
	modInfo.AddComponents([]CollectorModule{comp1, comp2})

	fullComponentsList := modInfo.GetFullComponentsList()
	assert.Len(t, fullComponentsList, 2)
	assert.Contains(t, fullComponentsList, comp1)
	assert.Contains(t, fullComponentsList, comp2)
}
