// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

const (
	testServiceInstanceID = "627cc493-f310-47de-96bd-71410b7dec09"
)

func TestFilterResourceAttributes(t *testing.T) {
	inputAttributes := map[string]any{
		"key.1": "val.1",
		"key.2": true,
		"key.3": 11,
		"key.4": "val.4",
	}
	for _, tc := range []struct {
		name                      string
		includeResourceAttributes []AttributeKeyValue
		expected                  map[string]any
	}{
		{
			name: "no_include_resource_attributes_configured",
			expected: map[string]any{
				// All resource attributes will be added as no
				// include resource attributes specified
				"key.1": "val.1",
				"key.2": true,
				"key.3": int64(11),
				"key.4": "val.4",
				// Collector instance info will be added
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
		{
			name: "include_resource_attributes_configured",
			includeResourceAttributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "key.1", false, nil),
				// Optional does not change the behavior for resource attribute
				// filtering.
				testAttributeKeyValue(t, "key.2", true, nil),
				// With default value configured and the attribute present, default
				// value will be ignored.
				testAttributeKeyValue(t, "key.3", false, 500),
				// With default value, even if the attribute is missing, it will
				// be added to the output.
				testAttributeKeyValue(t, "key.302", false, "anything"),
				// Without default value, the resource attribute will be ignored
				// if not present in the input
				testAttributeKeyValue(t, "key.404", false, nil),
				// Optional does not change the behavior for resource attribute
				// filtering.
				testAttributeKeyValue(t, "key.412", true, nil),
			},
			expected: map[string]any{
				// Include resource attributes are filtered
				"key.1": "val.1",
				// Optional attributes are copied if present
				"key.2": true,
				// Default value is ignored if attribute is present
				"key.3": int64(11),
				// Resource attributes with default values are added
				"key.302": "anything",
				// Collector instance info will be added
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := MetricDef[*ottlspan.TransformContext]{
				IncludeResourceAttributes: tc.includeResourceAttributes,
			}
			inputResourceAttrsM := pcommon.NewMap()
			require.NoError(t, inputResourceAttrsM.FromRaw(inputAttributes))
			actual, err := md.FilterResourceAttributes(
				context.Background(),
				(*ottlspan.TransformContext)(nil),
				inputResourceAttrsM,
				testCollectorInstanceInfo(t),
				zap.NewNop(),
			)
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(tc.expected, actual.AsRaw()))
		})
	}
}

func TestFilterAttributes(t *testing.T) {
	inputAttributes := map[string]any{
		"key.1": "val.1",
		"key.2": true,
		"key.3": 11,
		"key.4": "val.4",
	}
	for _, tc := range []struct {
		name               string
		attributes         []AttributeKeyValue
		expectedDecision   bool           // whether to process the entity or not
		expectedAttributes map[string]any // map of filtered attributes
	}{
		{
			name:               "no_attribute_configured",
			expectedDecision:   true,
			expectedAttributes: map[string]any{
				// No attributes are filtered since nothing is configured.
			},
		},
		{
			// Attribute filter would process an entity (span, log record,
			// datapoint) iff all the configured attributes, other than with
			// optional, are present. If any of the required attributes
			// are absent in the incoming entity then the entity is not
			// processed.
			name: "attributes_configured_but_missing",
			attributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "key.1", false, nil),
				// The below key has optional defined so should not
				// affect the processing decision.
				testAttributeKeyValue(t, "key.2", true, nil),
				// Below attribute would be missing in the input metric and
				// should return false to signal not to process the entity.
				testAttributeKeyValue(t, "key.404", false, nil),
			},
			expectedDecision: false,
		},
		{
			name: "attributes_configured_with_optional",
			attributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "key.1", false, nil),
				// The below two key has optional defined so should not
				// affect the processing decision.
				testAttributeKeyValue(t, "key.2", true, nil),
				testAttributeKeyValue(t, "key.412", true, nil),
			},
			expectedDecision: true,
			expectedAttributes: map[string]any{
				"key.1": "val.1",
				"key.2": true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := MetricDef[*ottlspan.TransformContext]{
				Attributes: tc.attributes,
			}
			inputAttrM := pcommon.NewMap()
			require.NoError(t, inputAttrM.FromRaw(inputAttributes))
			actual, ok := md.FilterAttributes(inputAttrM)
			assert.Equal(t, tc.expectedDecision, ok)
			if ok {
				// Only if the decision is true, the map should be compared
				assert.Empty(t, cmp.Diff(tc.expectedAttributes, actual.AsRaw()))
			}
		})
	}
}

func TestFilterResourceAttributes_DynamicExpression(t *testing.T) {
	parser := testSpanParser(t)

	resourceAttrs := map[string]any{
		"service.name":        "svc",
		"labels.team":         "platform",
		"labels.env":          "prod",
		"numeric_labels.cost": float64(42),
		"allowed.labels":      "team,cost",
	}

	cases := []struct {
		name                      string
		expr                      string
		includeResourceAttributes []AttributeKeyValue
		expected                  map[string]any
		wantErr                   string
	}{
		{
			name:    "dynamic_expression_non_map_returns_error",
			expr:    `"not_a_map"`,
			wantErr: "must return a pcommon.Map",
		},
		{
			name: "metadata_driven_attribute_selection",
			expr: `FilterMapByKeyList(resource.attributes, "team", ["labels.", "numeric_labels."])`,
			includeResourceAttributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "service.name", false, nil),
			},
			expected: map[string]any{
				"service.name":                          "svc",
				"labels.team":                           "platform",
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
		{
			name: "resource_attribute_driven_allow_list",
			expr: `FilterMapByKeyList(resource.attributes, resource.attributes["allowed.labels"], ["labels.", "numeric_labels."])`,
			includeResourceAttributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "service.name", false, nil),
			},
			expected: map[string]any{
				"service.name":                          "svc",
				"labels.team":                           "platform",
				"numeric_labels.cost":                   float64(42),
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
		{
			name: "prefix_based_label_forwarding",
			expr: `FilterMapByKeyList(resource.attributes, "*", ["labels.", "numeric_labels."])`,
			includeResourceAttributes: []AttributeKeyValue{
				testAttributeKeyValue(t, "service.name", false, nil),
			},
			expected: map[string]any{
				"service.name":                          "svc",
				"labels.team":                           "platform",
				"labels.env":                            "prod",
				"numeric_labels.cost":                   float64(42),
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dynExpr, err := parser.ParseValueExpression(tc.expr)
			require.NoError(t, err)

			md := MetricDef[*ottlspan.TransformContext]{
				Key:                       MetricKey{Name: "test.metric"},
				IncludeResourceAttributes: tc.includeResourceAttributes,
				DynResAttrs: &DynResAttrConfig[*ottlspan.TransformContext]{
					Expression: dynExpr,
				},
			}

			inputAttrs := pcommon.NewMap()
			require.NoError(t, inputAttrs.FromRaw(resourceAttrs))

			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			inputAttrs.CopyTo(rs.Resource().Attributes())
			ss := rs.ScopeSpans().AppendEmpty()
			span := ss.Spans().AppendEmpty()
			tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
			defer tCtx.Close()

			actual, err := md.FilterResourceAttributes(
				context.Background(),
				tCtx,
				inputAttrs,
				testCollectorInstanceInfo(t),
				zap.NewNop(),
			)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			if tc.expected != nil {
				assert.Empty(t, cmp.Diff(tc.expected, actual.AsRaw()))
			}
		})
	}
}

func testSpanParser(t *testing.T) ottl.Parser[*ottlspan.TransformContext] {
	t.Helper()
	set := componenttest.NewNopTelemetrySettings()
	funcs := customottl.SpanFuncs()
	parser, err := ottlspan.NewParser(funcs, set)
	require.NoError(t, err)
	return parser
}

func testCollectorInstanceInfo(t *testing.T) CollectorInstanceInfo {
	t.Helper()

	set := componenttest.NewNopTelemetrySettings()
	set.Resource.Attributes().PutStr("service.instance.id", testServiceInstanceID)
	return NewCollectorInstanceInfo(set)
}

func testAttributeKeyValue(
	t *testing.T,
	k string,
	optional bool,
	val any,
) AttributeKeyValue {
	t.Helper()

	defaultVal := pcommon.NewValueEmpty()
	if val != nil {
		require.NoError(t, defaultVal.FromRaw(val))
	}
	return AttributeKeyValue{
		Key:          k,
		Optional:     optional,
		DefaultValue: defaultVal,
	}
}
