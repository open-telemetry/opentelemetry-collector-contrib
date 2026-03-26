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
		includeResourceAttributes []attributeEntry[*ottlspan.TransformContext]
		expected                  map[string]any
	}{
		{
			name: "no_include_resource_attributes_configured",
			expected: map[string]any{
				"key.1":                                 "val.1",
				"key.2":                                 true,
				"key.3":                                 int64(11),
				"key.4":                                 "val.4",
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
		{
			name: "include_resource_attributes_configured",
			includeResourceAttributes: []attributeEntry[*ottlspan.TransformContext]{
				testAttributeEntry(t, "key.1", false, nil),
				testAttributeEntry(t, "key.2", true, nil),
				testAttributeEntry(t, "key.3", false, 500),
				testAttributeEntry(t, "key.302", false, "anything"),
				testAttributeEntry(t, "key.404", false, nil),
				testAttributeEntry(t, "key.412", true, nil),
			},
			expected: map[string]any{
				"key.1":                                 "val.1",
				"key.2":                                 true,
				"key.3":                                 int64(11),
				"key.302":                               "anything",
				"signal_to_metrics.service.instance.id": testServiceInstanceID,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := MetricDef[*ottlspan.TransformContext]{
				includeResourceAttributes: tc.includeResourceAttributes,
			}
			inputResourceAttrsM := pcommon.NewMap()
			require.NoError(t, inputResourceAttrsM.FromRaw(inputAttributes))
			// For static-only tests, resolved list matches the entries directly.
			resolved, err := md.ResolveIncludeResourceAttributes(t.Context(), nil)
			require.NoError(t, err)
			actual := md.FilterResourceAttributes(
				inputResourceAttrsM,
				resolved,
				testCollectorInstanceInfo(t),
			)
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
		attributes         []attributeEntry[*ottlspan.TransformContext]
		expectedDecision   bool
		expectedAttributes map[string]any
	}{
		{
			name:               "no_attribute_configured",
			expectedDecision:   true,
			expectedAttributes: map[string]any{},
		},
		{
			name: "attributes_configured_but_missing",
			attributes: []attributeEntry[*ottlspan.TransformContext]{
				testAttributeEntry(t, "key.1", false, nil),
				testAttributeEntry(t, "key.2", true, nil),
				testAttributeEntry(t, "key.404", false, nil),
			},
			expectedDecision: false,
		},
		{
			name: "attributes_configured_with_optional",
			attributes: []attributeEntry[*ottlspan.TransformContext]{
				testAttributeEntry(t, "key.1", false, nil),
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
				attributes: tc.attributes,
			}
			inputAttrM := pcommon.NewMap()
			require.NoError(t, inputAttrM.FromRaw(inputAttributes))
			ok := md.MatchAttributes(inputAttrM)
			assert.Equal(t, tc.expectedDecision, ok)
			if ok {
				resolved, err := md.ResolveAttributes(t.Context(), nil)
				require.NoError(t, err)
				actual := md.FilterAttributes(inputAttrM, resolved)
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
) attributeEntry[*ottlspan.TransformContext] {
	t.Helper()

	defaultVal := pcommon.NewValueEmpty()
	if val != nil {
		require.NoError(t, defaultVal.FromRaw(val))
	}
	return attributeEntry[*ottlspan.TransformContext]{
		Key:          k,
		Optional:     optional,
		DefaultValue: defaultVal,
	}
}
