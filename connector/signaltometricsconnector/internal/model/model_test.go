// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

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
		includeResourceAttributes []AttributeEntry[*ottlspan.TransformContext]
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
			includeResourceAttributes: []AttributeEntry[*ottlspan.TransformContext]{
				testAttributeEntry(t, "key.1", false, nil),
				// Optional does not change the behavior for resource attribute
				// filtering.
				testAttributeEntry(t, "key.2", true, nil),
				// With default value configured and the attribute present, default
				// value will be ignored.
				testAttributeEntry(t, "key.3", false, 500),
				// With default value, even if the attribute is missing, it will
				// be added to the output.
				testAttributeEntry(t, "key.302", false, "anything"),
				// Without default value, the resource attribute will be ignored
				// if not present in the input
				testAttributeEntry(t, "key.404", false, nil),
				// Optional does not change the behavior for resource attribute
				// filtering.
				testAttributeEntry(t, "key.412", true, nil),
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
				t.Context(),
				nil, // no transform context needed for static key tests
				inputResourceAttrsM,
				testCollectorInstanceInfo(t),
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
		attributes         []AttributeEntry[*ottlspan.TransformContext]
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
			attributes: []AttributeEntry[*ottlspan.TransformContext]{
				testAttributeEntry(t, "key.1", false, nil),
				// The below key has optional defined so should not
				// affect the processing decision.
				testAttributeEntry(t, "key.2", true, nil),
				// Below attribute would be missing in the input metric and
				// should return false to signal not to process the entity.
				testAttributeEntry(t, "key.404", false, nil),
			},
			expectedDecision: false,
		},
		{
			name: "attributes_configured_with_optional",
			attributes: []AttributeEntry[*ottlspan.TransformContext]{
				testAttributeEntry(t, "key.1", false, nil),
				// The below two key has optional defined so should not
				// affect the processing decision.
				testAttributeEntry(t, "key.2", true, nil),
				testAttributeEntry(t, "key.412", true, nil),
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
			ok := md.MatchAttributes(inputAttrM)
			assert.Equal(t, tc.expectedDecision, ok)
			if ok {
				actual, err := md.FilterAttributes(t.Context(), nil, inputAttrM)
				require.NoError(t, err)
				assert.Empty(t, cmp.Diff(tc.expectedAttributes, actual.AsRaw()))
			}
		})
	}
}

func TestExtractStringSlice(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    any
		expected []string
		wantErr  bool
	}{
		{
			name: "pcommon_slice",
			input: func() pcommon.Slice {
				s := pcommon.NewSlice()
				s.AppendEmpty().SetStr("labels.foo")
				s.AppendEmpty().SetStr("labels.bar")
				return s
			}(),
			expected: []string{"labels.foo", "labels.bar"},
		},
		{
			name:     "string_slice",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name: "pcommon_slice_non_string_element",
			input: func() pcommon.Slice {
				s := pcommon.NewSlice()
				s.AppendEmpty().SetInt(42)
				return s
			}(),
			wantErr: true,
		},
		{
			name:    "unsupported_type",
			input:   42,
			wantErr: true,
		},
		{
			name:     "empty_pcommon_slice",
			input:    pcommon.NewSlice(),
			expected: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := extractStringSlice(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func testCollectorInstanceInfo(t *testing.T) CollectorInstanceInfo {
	t.Helper()

	set := componenttest.NewNopTelemetrySettings()
	set.Resource.Attributes().PutStr("service.instance.id", testServiceInstanceID)
	return NewCollectorInstanceInfo(set)
}

func testAttributeEntry(
	t *testing.T,
	k string,
	optional bool,
	val any,
) AttributeEntry[*ottlspan.TransformContext] {
	t.Helper()

	defaultVal := pcommon.NewValueEmpty()
	if val != nil {
		require.NoError(t, defaultVal.FromRaw(val))
	}
	return AttributeEntry[*ottlspan.TransformContext]{
		Key:          k,
		Optional:     optional,
		DefaultValue: defaultVal,
	}
}
