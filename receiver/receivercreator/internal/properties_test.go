// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestParserEBNF(t *testing.T) {
	// ensures that all grammar changes are intentional
	require.Equal(t, `Property = (("io" <dot> "opentelemetry" <dot> "collector" <dot> "receiver-creator" <dot>) | ("receiver-creator" <dot> "collector" <dot> "opentelemetry" <dot> "io" <forwardslash>)) CID (((<dot> (("config" | "resource_attributes") <dot>?)) | (<dot> "rule")) | "") (<string> | <dot> | <forwardslash>)* .
CID = ~(<forwardslash> | (<dot> (?= ("config" | "rule" | "resource_attributes"))))+ (<forwardslash> ~(<dot> (?= ("config" | "rule" | "resource_attributes")))+*)? .`, parser.String())
}

func TestProperties(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		key                     string
		value                   string
		expectedType            string
		expectedMap             map[string]any
		expectedComponentIDType string
		expectedComponentIDName string
		expectedError           string
		expectedIsProp          bool
	}{
		{
			name:           "invalid config property",
			key:            "not a property",
			value:          "some value",
			expectedMap:    nil,
			expectedError:  `invalid property "not a property" (parsing error): receiver_creator:1:1: unexpected token "not a property"`,
			expectedIsProp: false,
		},
		{
			name:                    "standard config separator form map",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type.config",
			value:                   "one::two: null",
			expectedType:            ConfigType,
			expectedMap:             map[string]any{"one": map[string]any{"two": nil}},
			expectedComponentIDType: "receiver-type",
			expectedIsProp:          true,
		},
		{
			name:                    "config map with field in key",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type.config.one",
			value:                   "two::three.a: some.three.value",
			expectedType:            ConfigType,
			expectedMap:             map[string]any{"one": map[string]any{"two": map[string]any{"three.a": "some.three.value"}}},
			expectedComponentIDType: "receiver-type",
			expectedIsProp:          true,
		},
		{
			name:                    "standard resource attr",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type.resource_attributes",
			value:                   "attr.one: attr.one.value",
			expectedType:            ResourceAttributesType,
			expectedMap:             map[string]any{"attr.one": "attr.one.value"},
			expectedComponentIDType: "receiver-type",
			expectedIsProp:          true,
		},
		{
			name:                    "resource attr in key",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type/receiver-name.resource_attributes.attr.two",
			value:                   "attr.two.value",
			expectedType:            ResourceAttributesType,
			expectedMap:             map[string]any{"attr.two": "attr.two.value"},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name",
			expectedIsProp:          true,
		},
		{
			name:                    "resource attr mapping",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type/receiver-name.resource_attributes",
			value:                   `{"attr.one": "attr.one.value", "attr.two": "attr.two.value"}`,
			expectedType:            ResourceAttributesType,
			expectedMap:             map[string]any{"attr.one": "attr.one.value", "attr.two": "attr.two.value"},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name",
			expectedIsProp:          true,
		},
		{
			name:                    "standard rule",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type.rule",
			value:                   `type == "some.endpoint.type"`,
			expectedType:            RuleType,
			expectedMap:             map[string]any{"rule": `type == "some.endpoint.type"`},
			expectedComponentIDType: "receiver-type",
			expectedIsProp:          true,
		},
		{
			name:                    "__ rule",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type__receiver-name.rule",
			value:                   `type == "another.endpoint.type"`,
			expectedType:            RuleType,
			expectedMap:             map[string]any{"rule": `type == "another.endpoint.type"`},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name",
			expectedIsProp:          true,
		},
		{
			name:                    "__ resource attr in key",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type__receiver-name.resource_attributes.attr.one",
			value:                   "2.34567",
			expectedType:            ResourceAttributesType,
			expectedMap:             map[string]any{"attr.one": 2.34567},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name",
			expectedIsProp:          true,
		},
		{
			name:                    "config yaml value with name",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type__receiver-name.config",
			value:                   `{"one": {"two": {"three": "some value"}}}`,
			expectedMap:             map[string]any{"one": map[string]any{"two": map[string]any{"three": "some value"}}},
			expectedType:            ConfigType,
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name",
			expectedIsProp:          true,
		},
		{
			name:                    "config separator value with name ends in __x_y",
			key:                     "receiver-creator.collector.opentelemetry.io/receiver-type__receiver-name__x_y.config",
			value:                   "one::two::three: some value",
			expectedType:            ConfigType,
			expectedMap:             map[string]any{"one": map[string]any{"two": map[string]any{"three": "some value"}}},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name/x_y",
			expectedIsProp:          true,
		},
		{
			name: "config literal",
			key:  "receiver-creator.collector.opentelemetry.io/receiver-type__receiver-name.config",
			value: `one:
  two:
   nested: true
  three: some value
`,
			expectedType:            ConfigType,
			expectedMap:             map[string]any{"one": map[string]any{"two": map[string]any{"nested": true}, "three": "some value"}},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "receiver-name",
			expectedIsProp:          true,
		},
		{
			name: "full mapping property",
			key:  "receiver-creator.collector.opentelemetry.io/receiver-type",
			value: `config:
  one: one.value
  two: two.value
rule: type == "k8s.node"
resource_attributes:
  attr.one: attr_one_value
  attr.two: attr_two_value`,
			expectedType: "",
			expectedMap: map[string]any{
				"config": map[string]any{
					"one": "one.value",
					"two": "two.value",
				},
				"rule": `type == "k8s.node"`,
				"resource_attributes": map[string]any{
					"attr.one": "attr_one_value",
					"attr.two": "attr_two_value",
				},
			},
			expectedComponentIDType: "receiver-type",
			expectedIsProp:          true,
		},
		{
			name: "partial full mapping property with name",
			key:  "receiver-creator.collector.opentelemetry.io/receiver-type__some-receiver-name",
			value: `config:
  one: one.value
  two: two.value
resource_attributes:
  attr.one: attr_one_value
  attr.two: attr_two_value`,
			expectedType: "",
			expectedMap: map[string]any{
				"config": map[string]any{
					"one": "one.value",
					"two": "two.value",
				},
				"resource_attributes": map[string]any{
					"attr.one": "attr_one_value",
					"attr.two": "attr_two_value",
				},
			},
			expectedComponentIDType: "receiver-type",
			expectedComponentIDName: "some-receiver-name",
			expectedIsProp:          true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			prop, isProp, err := newProperty(tt.key, tt.value)
			require.Equal(t, tt.expectedIsProp, isProp)
			if tt.expectedIsProp {
				require.NotNil(t, prop, fmt.Errorf("unexpectedly nil prop: %w", err))
				assert.Equal(t, tt.expectedType, prop.Type)
			} else {
				assert.Nil(t, prop)
			}
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.key, prop.Raw)
			}
			assert.Equal(t, tt.expectedMap, prop.toStringMap())
			assert.Equal(t, tt.expectedComponentIDType, string(prop.componentID().Type()))
			assert.Equal(t, tt.expectedComponentIDName, prop.componentID().Name())
		})
	}
}

func TestParsePropertiesFromEnv(t *testing.T) {
	k8sAnnotations := map[string]string{
		"receiver-creator.collector.opentelemetry.io/another-receiver-type.rule":        "type == \"another.endpoint.type\"",
		"receiver-creator.collector.opentelemetry.io/receiver-type":                     "config::one: should be overridden",
		"receiver-creator.collector.opentelemetry.io/receiver-type.config":              "one::two: null",
		"receiver-creator.collector.opentelemetry.io/receiver-type.config.one":          "two::three.b: 1.2345",
		"receiver-creator.collector.opentelemetry.io/receiver-type.resource_attributes": "attr.two: attr.two.value",
		"receiver-creator.collector.opentelemetry.io/receiver-type__name.rule":          "type == \"another.endpoint.type\"",
	}

	for k := range k8sAnnotations {
		invalidKey := validation.IsQualifiedName(k)
		assert.Zero(t, len(invalidKey), "invalid label key %q: %v", k, invalidKey)
	}

	dockerLabels := map[string]string{
		"not.a.property": "some.label.value",
		"io.opentelemetry.collector.receiver-creator.invalid":                                    "invalid::::  invalid",
		"io.opentelemetry.collector.receiver-creator.invalid/yaml":                               `config: { """: invalid - true }`,
		"io.opentelemetry.collector.receiver-creator.receiver-type":                              "config::one::two: should by overridden",
		"io.opentelemetry.collector.receiver-creator.receiver-type.config.one":                   "two::three.a: some.three.value",
		"io.opentelemetry.collector.receiver-creator.receiver-type.resource_attributes.attr.one": "attr.one.value",
		"io.opentelemetry.collector.receiver-creator.receiver-type.rule":                         "type == \"some.endpoint.type\"",
		"io.opentelemetry.collector.receiver-creator.receiver-type__name.config.one":             "two::three: another.three.value",
		"io.opentelemetry.collector.receiver-creator.receiver-type__name.resource_attributes":    "attr.one: 2.34567",
	}

	properties := map[string]string{}

	for _, l := range []map[string]string{k8sAnnotations, dockerLabels} {
		for k, v := range l {
			properties[k] = v
		}
	}

	for _, details := range []observer.EndpointDetails{
		&observer.Container{Labels: properties},
		&observer.Pod{Annotations: properties},
		&observer.Port{Pod: observer.Pod{Annotations: properties}},
		&observer.K8sNode{Annotations: properties},
	} {
		t.Run(string(details.Type()), func(t *testing.T) {
			zc, observedLogs := zapobserver.New(zap.DebugLevel)
			// determinism check
			for i := 0; i < 100; i++ {
				parsed := PropertyConfFromEndpointEnv(details, zap.New(zc))
				require.Equal(t, map[string]any{
					"another-receiver-type": map[string]any{
						"rule": "type == \"another.endpoint.type\"",
					},
					"receiver-type": map[string]any{
						"config": map[string]any{
							"one": map[string]any{
								"two": map[string]any{
									"three.a": "some.three.value",
									"three.b": 1.2345,
								},
							},
						},
						"resource_attributes": map[string]any{
							"attr.one": "attr.one.value",
							"attr.two": "attr.two.value",
						},
						"rule": "type == \"some.endpoint.type\"",
					},
					"receiver-type/name": map[string]any{
						"config": map[string]any{
							"one": map[string]any{
								"two": map[string]any{
									"three": "another.three.value",
								},
							},
						},
						"resource_attributes": map[string]any{
							"attr.one": 2.34567,
						},
						"rule": "type == \"another.endpoint.type\"",
					},
				}, parsed.ToStringMap())
			}
			// confirm only expected error messages logged
			var invalid, invalidYaml int
			for _, m := range observedLogs.All() {
				require.Equal(t, "error creating property", m.Message)
				cm := m.ContextMap()
				require.Contains(t, cm, "parsed")
				switch parsed := cm["parsed"].(string); parsed {
				case "io.opentelemetry.collector.receiver-creator.invalid":
					invalid++
					require.Contains(t, cm, "error")
					require.Equal(t, `invalid endpoint property keys: [invalid]. Must be in ["config", "resource_attributes", "rule"]`, cm["error"])
				case "io.opentelemetry.collector.receiver-creator.invalid/yaml":
					invalidYaml++
					require.Contains(t, cm, "error")
					require.Equal(t, `failed unmarshaling full mapping property ("io.opentelemetry.collector.receiver-creator.invalid/yaml") -> yaml: found unexpected end of stream`, cm["error"])
				default:
					t.Fatalf("unexpected parse value %q", parsed)
				}
			}
			require.Equal(t, 100, invalid)
			require.Equal(t, 100, invalidYaml)
			require.Equal(t, 200, observedLogs.Len())
		})
	}
}
