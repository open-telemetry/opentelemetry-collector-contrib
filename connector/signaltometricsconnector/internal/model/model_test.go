// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

const (
	testServiceInstanceID = "627cc493-f310-47de-96bd-71410b7dec09"
)

// testMetricDef creates a MetricDef via the real FromMetricInfo path
// with the given include_resource_attributes and attributes config.
func testMetricDef(
	t *testing.T,
	includeResAttrs []config.Attribute,
	attrs []config.Attribute,
) MetricDef[*ottlspan.TransformContext] {
	t.Helper()

	parser, err := ottlspan.NewParser(
		customottl.SpanFuncs(),
		componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)

	mi := config.MetricInfo{
		Name:                      "test.metric",
		IncludeResourceAttributes: includeResAttrs,
		Attributes:                attrs,
		Sum:                       configoptional.Some(config.Sum{Value: "1"}),
	}
	var md MetricDef[*ottlspan.TransformContext]
	require.NoError(t, md.FromMetricInfo(mi, parser, componenttest.NewNopTelemetrySettings()))
	return md
}

func TestFilterResourceAttributes(t *testing.T) {
	inputAttributes := map[string]any{
		"key.1": "val.1",
		"key.2": true,
		"key.3": 11,
		"key.4": "val.4",
	}
	for _, tc := range []struct {
		name                      string
		includeResourceAttributes []config.Attribute
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
			includeResourceAttributes: []config.Attribute{
				{Key: "key.1"},
				{Key: "key.2", Optional: true},
				{Key: "key.3", DefaultValue: 500},
				{Key: "key.302", DefaultValue: "anything"},
				{Key: "key.404"},
				{Key: "key.412", Optional: true},
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
			md := testMetricDef(t, tc.includeResourceAttributes, nil)
			inputResourceAttrsM := pcommon.NewMap()
			require.NoError(t, inputResourceAttrsM.FromRaw(inputAttributes))
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
		attributes         []config.Attribute
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
			attributes: []config.Attribute{
				{Key: "key.1"},
				{Key: "key.2", Optional: true},
				{Key: "key.404"},
			},
			expectedDecision: false,
		},
		{
			name: "attributes_configured_with_optional",
			attributes: []config.Attribute{
				{Key: "key.1"},
				{Key: "key.2", Optional: true},
				{Key: "key.412", Optional: true},
			},
			expectedDecision: true,
			expectedAttributes: map[string]any{
				"key.1": "val.1",
				"key.2": true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md := testMetricDef(t, nil, tc.attributes)
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
