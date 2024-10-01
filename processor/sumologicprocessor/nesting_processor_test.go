// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestNestingAttributes(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]pcommon.Value
		expected map[string]pcommon.Value
		include  []string
		exclude  []string
	}{
		{
			name: "sample nesting",
			input: map[string]pcommon.Value{
				"kubernetes.container_name": pcommon.NewValueStr("xyz"),
				"kubernetes.host.name":      pcommon.NewValueStr("the host"),
				"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
				"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
				"another_attr":              pcommon.NewValueStr("42"),
			},
			expected: map[string]pcommon.Value{
				"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
					"container_name": pcommon.NewValueStr("xyz"),
					"namespace_name": pcommon.NewValueStr("sumologic"),
					"host": mapToPcommonValue(map[string]pcommon.Value{
						"name":    pcommon.NewValueStr("the host"),
						"address": pcommon.NewValueStr("127.0.0.1"),
					}),
				}),
				"another_attr": pcommon.NewValueStr("42"),
			},
			include: []string{},
			exclude: []string{},
		},
		{
			name: "single values",
			input: map[string]pcommon.Value{
				"a": mapToPcommonValue(map[string]pcommon.Value{
					"b": mapToPcommonValue(map[string]pcommon.Value{
						"c": pcommon.NewValueStr("d"),
					}),
				}),
				"a.b.c": pcommon.NewValueStr("d"),
				"d.g.e": pcommon.NewValueStr("l"),
				"b.g.c": pcommon.NewValueStr("bonus"),
			},
			expected: map[string]pcommon.Value{
				"a": mapToPcommonValue(map[string]pcommon.Value{
					"b": mapToPcommonValue(map[string]pcommon.Value{
						"c": pcommon.NewValueStr("d"),
					}),
				}),
				"d": mapToPcommonValue(map[string]pcommon.Value{
					"g": mapToPcommonValue(map[string]pcommon.Value{
						"e": pcommon.NewValueStr("l"),
					}),
				}),
				"b": mapToPcommonValue(map[string]pcommon.Value{
					"g": mapToPcommonValue(map[string]pcommon.Value{
						"c": pcommon.NewValueStr("bonus"),
					}),
				}),
			},
			include: []string{},
			exclude: []string{},
		},
		{
			name: "overwrite map with simple value",
			input: map[string]pcommon.Value{
				"sumo.logic": pcommon.NewValueBool(true),
				"sumo":       pcommon.NewValueBool(false),
			},
			expected: map[string]pcommon.Value{
				"sumo": mapToPcommonValue(map[string]pcommon.Value{
					"":      pcommon.NewValueBool(false),
					"logic": pcommon.NewValueBool(true),
				}),
			},
			include: []string{},
			exclude: []string{},
		},
		{
			name: "allowlist",
			input: map[string]pcommon.Value{
				"kubernetes.container_name": pcommon.NewValueStr("xyz"),
				"kubernetes.host.name":      pcommon.NewValueStr("the host"),
				"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
				"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
				"another_attr":              pcommon.NewValueStr("42"),
			},
			expected: map[string]pcommon.Value{
				"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
					"container_name": pcommon.NewValueStr("xyz"),
					"host": mapToPcommonValue(map[string]pcommon.Value{
						"name": pcommon.NewValueStr("the host"),
					}),
				}),
				"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
				"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
				"another_attr":              pcommon.NewValueStr("42"),
			},
			include: []string{"kubernetes.container", "kubernetes.host.name"},
			exclude: []string{},
		},
		{
			name: "denylist",
			input: map[string]pcommon.Value{
				"kubernetes.container_name": pcommon.NewValueStr("xyz"),
				"kubernetes.host.name":      pcommon.NewValueStr("the host"),
				"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
				"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
				"another_attr":              pcommon.NewValueStr("42"),
			},
			expected: map[string]pcommon.Value{
				"kubernetes.container_name": pcommon.NewValueStr("xyz"),
				"kubernetes.host.name":      pcommon.NewValueStr("the host"),
				"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
				"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
					"namespace_name": pcommon.NewValueStr("sumologic"),
				}),
				"another_attr": pcommon.NewValueStr("42"),
			},
			include: []string{},
			exclude: []string{"kubernetes.container", "kubernetes.host"},
		},
		{
			name: "denylist and allowlist",
			input: map[string]pcommon.Value{
				"kubernetes.container_name":         pcommon.NewValueStr("xyz"),
				"kubernetes.host.name":              pcommon.NewValueStr("the host"),
				"kubernetes.host.naming_convention": pcommon.NewValueStr("random"),
				"kubernetes.host.address":           pcommon.NewValueStr("127.0.0.1"),
				"kubernetes.namespace_name":         pcommon.NewValueStr("sumologic"),
				"another_attr":                      pcommon.NewValueStr("42"),
				"and_end":                           pcommon.NewValueStr("fin"),
			},
			expected: map[string]pcommon.Value{
				"kubernetes.container_name":         pcommon.NewValueStr("xyz"),
				"kubernetes.host.naming_convention": pcommon.NewValueStr("random"),
				"kubernetes.namespace_name":         pcommon.NewValueStr("sumologic"),
				"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
					"host": mapToPcommonValue(map[string]pcommon.Value{
						"name":    pcommon.NewValueStr("the host"),
						"address": pcommon.NewValueStr("127.0.0.1"),
					}),
				}),
				"another_attr": pcommon.NewValueStr("42"),
				"and_end":      pcommon.NewValueStr("fin"),
			},
			include: []string{"kubernetes.host."},
			exclude: []string{"kubernetes.host.naming"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proc := newNestingProcessor(&NestingProcessorConfig{
				Separator: ".", Enabled: true, Include: testCase.include, Exclude: testCase.exclude, SquashSingleValues: false,
			})

			attrs := mapToPcommonMap(testCase.input)
			err := proc.processAttributes(attrs)
			require.NoError(t, err)

			expected := mapToPcommonMap(testCase.expected)

			require.Equal(t, expected.AsRaw(), attrs.AsRaw())
		})
	}
}

func TestSquashing(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]pcommon.Value
		expected map[string]pcommon.Value
	}{
		{
			name: "squash from example",
			input: map[string]pcommon.Value{
				"k8s": mapToPcommonValue(map[string]pcommon.Value{
					"pods": mapToPcommonValue(map[string]pcommon.Value{
						"a": pcommon.NewValueStr("A"),
						"b": pcommon.NewValueStr("B"),
					}),
				}),
			},
			expected: map[string]pcommon.Value{
				"k8s.pods": mapToPcommonValue(map[string]pcommon.Value{
					"a": pcommon.NewValueStr("A"),
					"b": pcommon.NewValueStr("B"),
				}),
			},
		},
		{
			name: "many-value maps with squashed keys",
			input: map[string]pcommon.Value{
				"k8s": mapToPcommonValue(map[string]pcommon.Value{
					"pods": mapToPcommonValue(map[string]pcommon.Value{
						"a": mapToPcommonValue(map[string]pcommon.Value{
							"b": mapToPcommonValue(map[string]pcommon.Value{
								"c": pcommon.NewValueStr("A"),
							}),
						}),
						"b": pcommon.NewValueStr("B"),
					}),
				}),
				"sumo": mapToPcommonValue(map[string]pcommon.Value{
					"logic": mapToPcommonValue(map[string]pcommon.Value{
						"schema": pcommon.NewValueStr("processor"),
					}),
				}),
			},
			expected: map[string]pcommon.Value{
				"k8s.pods": mapToPcommonValue(map[string]pcommon.Value{
					"a.b.c": pcommon.NewValueStr("A"),
					"b":     pcommon.NewValueStr("B"),
				}),
				"sumo.logic.schema": pcommon.NewValueStr("processor"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proc := newNestingProcessor(&NestingProcessorConfig{
				Separator: ".", Enabled: true, Include: []string{}, Exclude: []string{}, SquashSingleValues: true,
			})

			attrs := mapToPcommonMap(testCase.input)
			err := proc.processAttributes(attrs)
			require.NoError(t, err)

			expected := mapToPcommonMap(testCase.expected)

			require.Equal(t, expected.AsRaw(), attrs.AsRaw())
		})
	}
}
