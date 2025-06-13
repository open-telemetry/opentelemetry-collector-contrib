// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package componentchecker // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
)

func TestDataToFlattenedJSONString(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected string
	}{
		{
			name: "Simple map without lines",
			data: map[string]any{
				"key": "value",
			},
			expected: `{"key":"value"}`,
		},
		{
			name:     "Invalid JSON",
			data:     make(chan int),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DataToFlattenedJSONString(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPopulateFullComponentsJSON(t *testing.T) {
	tests := []struct {
		name                     string
		moduleInfo               service.ModuleInfos
		collectorConfigStringMap map[string]any
		components               []payload.CollectorModule
	}{
		{
			name: "All component types included",
			moduleInfo: service.ModuleInfos{
				Receiver: map[component.Type]service.ModuleInfo{
					component.MustNewType("examplereceiver"): {BuilderRef: "example.com/module v1.0.0"},
				},
				Processor: map[component.Type]service.ModuleInfo{
					component.MustNewType("exampleprocessor"): {BuilderRef: "example.com/module v1.0.0"},
				},
				Exporter: map[component.Type]service.ModuleInfo{
					component.MustNewType("exampleexporter"): {BuilderRef: "example.com/module v1.0.0"},
				},
				Extension: map[component.Type]service.ModuleInfo{
					component.MustNewType("exampleextension"): {BuilderRef: "example.com/module v1.0.0"},
				},
				Connector: map[component.Type]service.ModuleInfo{
					component.MustNewType("exampleconnector"): {BuilderRef: "example.com/module v1.0.0"},
				},
			},
			collectorConfigStringMap: map[string]any{
				"receivers": map[string]any{
					"examplereceiver": map[string]any{},
				},
				"processors": map[string]any{
					"exampleprocessor": map[string]any{},
				},
				"exporters": map[string]any{
					"exampleexporter": map[string]any{},
				},
				"extensions": map[string]any{
					"exampleextension": map[string]any{},
				},
				"connectors": map[string]any{
					"exampleconnector": map[string]any{},
				},
			},
			components: []payload.CollectorModule{
				{
					Type:       "examplereceiver",
					Kind:       "receiver",
					Gomod:      "example.com/module",
					Version:    "v1.0.0",
					Configured: true,
				},
				{
					Type:       "exampleprocessor",
					Kind:       "processor",
					Gomod:      "example.com/module",
					Version:    "v1.0.0",
					Configured: true,
				},
				{
					Type:       "exampleexporter",
					Kind:       "exporter",
					Gomod:      "example.com/module",
					Version:    "v1.0.0",
					Configured: true,
				},
				{
					Type:       "exampleextension",
					Kind:       "extension",
					Gomod:      "example.com/module",
					Version:    "v1.0.0",
					Configured: true,
				},
				{
					Type:       "exampleconnector",
					Kind:       "connector",
					Gomod:      "example.com/module",
					Version:    "v1.0.0",
					Configured: true,
				},
			},
		},
		{
			name: "No components included",
			moduleInfo: service.ModuleInfos{
				Receiver:  map[component.Type]service.ModuleInfo{},
				Processor: map[component.Type]service.ModuleInfo{},
				Exporter:  map[component.Type]service.ModuleInfo{},
				Extension: map[component.Type]service.ModuleInfo{},
				Connector: map[component.Type]service.ModuleInfo{},
			},
			collectorConfigStringMap: map[string]any{},
			components:               []payload.CollectorModule{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := confmap.NewFromStringMap(tt.collectorConfigStringMap)
			modInfo, _ := PopulateFullComponentsJSON(tt.moduleInfo, c)
			assert.ElementsMatch(t, tt.components, modInfo.GetFullComponentsList())
		})
	}
}

func TestPopulateActiveComponents(t *testing.T) {
	tests := []struct {
		name                     string
		collectorConfigStringMap map[string]any
		moduleInfoJSON           *payload.ModuleInfoJSON
		expectedComponents       []payload.ServiceComponent
		expectedError            string
	}{
		{
			name: "All component types included",
			collectorConfigStringMap: map[string]any{
				"receivers": map[string]any{
					"examplereceiver": map[string]any{},
				},
				"processors": map[string]any{
					"exampleprocessor": map[string]any{},
				},
				"exporters": map[string]any{
					"exampleexporter": map[string]any{},
				},
				"extensions": map[string]any{
					"exampleextension": map[string]any{},
				},
				"service": map[string]any{
					"extensions": []any{
						"exampleextension",
					},
					"pipelines": map[string]any{
						"traces": map[string]any{
							"receivers": []any{
								"examplereceiver",
							},
							"processors": []any{
								"exampleprocessor",
							},
							"exporters": []any{
								"exampleexporter",
							},
						},
					},
				},
			},
			moduleInfoJSON: func() *payload.ModuleInfoJSON {
				mij := payload.NewModuleInfoJSON()
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleextension",
					Kind:    "extension",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "examplereceiver",
					Kind:    "receiver",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleprocessor",
					Kind:    "processor",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleexporter",
					Kind:    "exporter",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				return mij
			}(),
			expectedComponents: []payload.ServiceComponent{
				{
					ID:      "exampleextension",
					Name:    "",
					Type:    "exampleextension",
					Kind:    "extension",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				},
				{
					ID:       "examplereceiver",
					Name:     "",
					Type:     "examplereceiver",
					Kind:     "receiver",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "traces",
				},
				{
					ID:       "exampleprocessor",
					Name:     "",
					Type:     "exampleprocessor",
					Kind:     "processor",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "traces",
				},
				{
					ID:       "exampleexporter",
					Name:     "",
					Type:     "exampleexporter",
					Kind:     "exporter",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "traces",
				},
			},
			expectedError: "",
		},
		{
			name: "No service map",
			collectorConfigStringMap: map[string]any{
				"receivers": map[string]any{},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "",
		},
		{
			name: "Invalid extensions list",
			collectorConfigStringMap: map[string]any{
				"service": map[string]any{
					"extensions": map[string]any{},
				},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "'service.extensions': source data must be an array or slice, got map",
		},
		{
			name: "Invalid extension value",
			collectorConfigStringMap: map[string]any{
				"service": map[string]any{
					"extensions": []any{
						123,
					},
				},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "'service.extensions[0]' expected a map, got 'int'",
		},
		{
			name: "Invalid pipeline map",
			collectorConfigStringMap: map[string]any{
				"service": map[string]any{
					"pipelines": []any{},
				},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "'service.pipelines' expected a map, got 'slice'",
		},
		{
			name: "Invalid pipeline components map",
			collectorConfigStringMap: map[string]any{
				"service": map[string]any{
					"pipelines": map[string]any{
						"traces": []any{},
					},
				},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "'service.pipelines[traces]' expected a map, got 'slice'",
		},
		{
			name: "Invalid pipeline components list",
			collectorConfigStringMap: map[string]any{
				"service": map[string]any{
					"pipelines": map[string]any{
						"traces": map[string]any{
							"receivers": map[string]any{},
						},
					},
				},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "'service.pipelines[traces].receivers': source data must be an array or slice, got map",
		},
		{
			name: "Invalid pipeline component value",
			collectorConfigStringMap: map[string]any{
				"service": map[string]any{
					"pipelines": map[string]any{
						"traces": map[string]any{
							"receivers": []any{
								123,
							},
						},
					},
				},
			},
			moduleInfoJSON:     payload.NewModuleInfoJSON(),
			expectedComponents: []payload.ServiceComponent{},
			expectedError:      "'service.pipelines[traces].receivers[0]' expected a map, got 'int'",
		},
		{
			name: "Connector as exporter in traces and receiver in metrics",
			collectorConfigStringMap: map[string]any{
				"receivers": map[string]any{
					"examplereceiver": map[string]any{},
				},
				"processors": map[string]any{
					"exampleprocessor": map[string]any{},
				},
				"exporters": map[string]any{
					"exampleexporter": map[string]any{},
				},
				"extensions": map[string]any{
					"exampleextension": map[string]any{},
				},
				"connectors": map[string]any{
					"exampleconnector": map[string]any{},
				},
				"service": map[string]any{
					"extensions": []any{
						"exampleextension",
					},
					"pipelines": map[string]any{
						"traces": map[string]any{
							"receivers": []any{
								"examplereceiver",
							},
							"processors": []any{
								"exampleprocessor",
							},
							"exporters": []any{
								"exampleconnector",
							},
						},
						"metrics": map[string]any{
							"receivers": []any{
								"exampleconnector",
							},
							"processors": []any{
								"exampleprocessor",
							},
							"exporters": []any{
								"exampleexporter",
							},
						},
					},
				},
			},
			moduleInfoJSON: func() *payload.ModuleInfoJSON {
				mij := payload.NewModuleInfoJSON()
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleextension",
					Kind:    "extension",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "examplereceiver",
					Kind:    "receiver",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleprocessor",
					Kind:    "processor",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleexporter",
					Kind:    "exporter",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				mij.AddComponent(payload.CollectorModule{
					Type:    "exampleconnector",
					Kind:    "connector",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				})
				return mij
			}(),
			expectedComponents: []payload.ServiceComponent{
				{
					ID:      "exampleextension",
					Name:    "",
					Type:    "exampleextension",
					Kind:    "extension",
					Gomod:   "example.com/module",
					Version: "v1.0.0",
				},
				{
					ID:       "examplereceiver",
					Name:     "",
					Type:     "examplereceiver",
					Kind:     "receiver",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "traces",
				},
				{
					ID:       "exampleprocessor",
					Name:     "",
					Type:     "exampleprocessor",
					Kind:     "processor",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "traces",
				},
				{
					ID:       "exampleprocessor",
					Name:     "",
					Type:     "exampleprocessor",
					Kind:     "processor",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "metrics",
				},
				{
					ID:       "exampleconnector",
					Name:     "",
					Type:     "exampleconnector",
					Kind:     "connector",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "traces",
				},
				{
					ID:       "exampleconnector",
					Name:     "",
					Type:     "exampleconnector",
					Kind:     "connector",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "metrics",
				},
				{
					ID:       "exampleexporter",
					Name:     "",
					Type:     "exampleexporter",
					Kind:     "exporter",
					Gomod:    "example.com/module",
					Version:  "v1.0.0",
					Pipeline: "metrics",
				},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confMap := confmap.NewFromStringMap(tt.collectorConfigStringMap)
			activeComponents, err := PopulateActiveComponents(confMap, tt.moduleInfoJSON)
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedComponents, *activeComponents)
			}
		})
	}
}

func TestIsComponentConfigured(t *testing.T) {
	tests := []struct {
		name           string
		componentType  component.Type
		configMap      map[component.ID]component.Config
		expectedResult bool
	}{
		{
			name:          "Component type is configured",
			componentType: component.MustNewType("exampletype"),
			configMap: map[component.ID]component.Config{
				component.MustNewID("exampletype"): nil,
			},
			expectedResult: true,
		},
		{
			name:          "Component type is not configured",
			componentType: component.MustNewType("exampletype"),
			configMap: map[component.ID]component.Config{
				component.MustNewID("othertype"): nil,
			},
			expectedResult: false,
		},
		{
			name:           "Empty config map",
			componentType:  component.MustNewType("exampletype"),
			configMap:      map[component.ID]component.Config{},
			expectedResult: false,
		},
		{
			name:          "Multiple components, one matches",
			componentType: component.MustNewType("exampletype"),
			configMap: map[component.ID]component.Config{
				component.MustNewID("othertype"):   nil,
				component.MustNewID("exampletype"): nil,
			},
			expectedResult: true,
		},
		{
			name:          "Multiple components, none match",
			componentType: component.MustNewType("exampletype"),
			configMap: map[component.ID]component.Config{
				component.MustNewID("othertype1"): nil,
				component.MustNewID("othertype2"): nil,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isComponentConfigured(tt.componentType, tt.configMap)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPopulateFullComponentsJSONErrorPaths(t *testing.T) {
	t.Run("empty moduleInfos", func(t *testing.T) {
		// Test with empty moduleInfos - should handle gracefully, not error
		confMap := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{
				"examplereceiver": map[string]any{},
			},
		})

		// This should handle empty moduleInfos gracefully
		modInfo, err := PopulateFullComponentsJSON(service.ModuleInfos{}, confMap)
		require.NoError(t, err)
		assert.NotNil(t, modInfo)
		assert.Empty(t, modInfo.GetFullComponentsList()) // No components
	})

	t.Run("invalid config format", func(t *testing.T) {
		// Test with invalid config format to trigger unmarshal error
		moduleInfo := service.ModuleInfos{
			Receiver: map[component.Type]service.ModuleInfo{
				component.MustNewType("examplereceiver"): {BuilderRef: "example.com/module v1.0.0"},
			},
		}

		// Create invalid configuration that will fail unmarshal
		confMap := confmap.NewFromStringMap(map[string]any{
			"receivers": "invalid-format", // Should be a map, not string
		})

		_, err := PopulateFullComponentsJSON(moduleInfo, confMap)
		require.Error(t, err) // Should error on unmarshal
	})

	t.Run("module with only gomod, no version", func(t *testing.T) {
		// Test with module that has no version (single part after split)
		moduleInfo := service.ModuleInfos{
			Receiver: map[component.Type]service.ModuleInfo{
				component.MustNewType("examplereceiver"): {BuilderRef: "invalid-format"}, // No space, no version
			},
		}
		confMap := confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{
				"examplereceiver": map[string]any{},
			},
		})

		// This should panic due to index out of range when trying to access parts[1]
		assert.Panics(t, func() {
			_, err := PopulateFullComponentsJSON(moduleInfo, confMap)
			require.Error(t, err)
		})
	})
}

func TestPopulateActiveComponentsAdditionalErrorPaths(t *testing.T) {
	t.Run("nil moduleInfoJSON", func(t *testing.T) {
		confMap := confmap.NewFromStringMap(map[string]any{
			"service": map[string]any{
				"extensions": []any{"test"},
			},
		})

		// This should panic when trying to call GetComponent on nil moduleInfoJSON
		assert.Panics(t, func() {
			_, err := PopulateActiveComponents(confMap, nil)
			require.Error(t, err)
		})
	})

	t.Run("invalid config format", func(t *testing.T) {
		confMap := confmap.NewFromStringMap(map[string]any{
			"service": "invalid-format", // Should be a map, not string
		})

		moduleInfoJSON := payload.NewModuleInfoJSON()
		_, err := PopulateActiveComponents(confMap, moduleInfoJSON)
		require.Error(t, err) // Should error on unmarshal
	})

	t.Run("extension not found in module info", func(t *testing.T) {
		confMap := confmap.NewFromStringMap(map[string]any{
			"extensions": map[string]any{
				"missing_extension": map[string]any{},
			},
			"service": map[string]any{
				"extensions": []any{"missing_extension"},
			},
		})

		moduleInfoJSON := payload.NewModuleInfoJSON()
		// Don't add the extension to moduleInfoJSON

		_, err := PopulateActiveComponents(confMap, moduleInfoJSON)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "extension not found in Module Info")
	})
}

func TestDataToFlattenedJSONStringAdditionalCases(t *testing.T) {
	t.Run("complex nested structure", func(t *testing.T) {
		data := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"key":   "value",
					"array": []string{"item1", "item2"},
				},
				"simple": "test",
			},
			"number": 123,
		}

		result := DataToFlattenedJSONString(data)
		assert.NotEmpty(t, result)
		assert.NotContains(t, result, "\n")
		assert.NotContains(t, result, "\r")

		// Verify it's valid JSON
		var parsed map[string]any
		err := json.Unmarshal([]byte(result), &parsed)
		assert.NoError(t, err)
	})

	t.Run("nil input", func(t *testing.T) {
		result := DataToFlattenedJSONString(nil)
		assert.Equal(t, "null", result)
	})
}
