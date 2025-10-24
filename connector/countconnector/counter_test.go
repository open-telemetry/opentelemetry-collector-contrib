// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

func Test_update_attribute_inheritance(t *testing.T) {
	spanMetricDefs := make(map[string]metricDef[ottlspan.TransformContext])
	spanMetricDefs[defaultMetricNameSpans] = metricDef[ottlspan.TransformContext]{
		desc: defaultMetricDescSpans,
		attrs: []AttributeConfig{
			{
				Key:          "component",
				DefaultValue: "default",
			},
			{
				Key:          "version",
				DefaultValue: "default",
			},
		},
	}

	tests := []struct {
		name         string
		resourceAttr pcommon.Map
		scopeAttr    pcommon.Map
		spanAttr     pcommon.Map
		expectedAttr pcommon.Map
	}{
		{
			name:         "attributes from DefaultValue",
			resourceAttr: pcommon.NewMap(),
			scopeAttr:    pcommon.NewMap(),
			spanAttr:     pcommon.NewMap(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "default")
				res.PutStr("version", "default")
				return res
			}(),
		},
		{
			name: "attributes from resourceAttr",
			resourceAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "otelcol")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
			scopeAttr: pcommon.NewMap(),
			spanAttr:  pcommon.NewMap(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "otelcol")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
		},
		{
			name:         "attributes from scopeAttr",
			resourceAttr: pcommon.NewMap(),
			scopeAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "otelcol")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
			spanAttr: pcommon.NewMap(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "otelcol")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
		},
		{
			name:         "attributes from spanAttr",
			resourceAttr: pcommon.NewMap(),
			scopeAttr:    pcommon.NewMap(),
			spanAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "otelcol")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "otelcol")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
		},
		{
			name: "attributes with order: spanAttr > scopeAttr > resourceAttr",
			resourceAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "value-from-resourceAttr")
				res.PutStr("version", "v1.31.0")
				return res
			}(),
			scopeAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "value-from-scopeAttr")
				res.PutStr("version", "value-from-scopeAttr")
				return res
			}(),
			spanAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "value-from-spanAttr")
				return res
			}(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("component", "value-from-spanAttr")
				res.PutStr("version", "value-from-scopeAttr")
				return res
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spansCounter := newCounter[ottlspan.TransformContext](spanMetricDefs)
			err := spansCounter.update(t.Context(), tt.spanAttr, tt.scopeAttr, tt.resourceAttr, ottlspan.TransformContext{})
			require.NoError(t, err)
			require.NotNil(t, spansCounter)
			m := spansCounter.counts[defaultMetricNameSpans]
			expectKey := pdatautil.MapHash(tt.expectedAttr)
			attrCount, ok := m[expectKey]
			require.True(t, ok)
			require.NotNil(t, attrCount)
		})
	}
}

func Test_update_attributes_types(t *testing.T) {
	tests := []struct {
		name         string
		config       []AttributeConfig
		inputAttr    pcommon.Map
		expectedAttr pcommon.Map
	}{
		{
			name: "attributes from DefaultValue",
			config: []AttributeConfig{
				{
					Key:          "string",
					DefaultValue: "default-string",
				},
				{
					Key:          "int",
					DefaultValue: 123,
				},
				{
					Key:          "double",
					DefaultValue: 321.0,
				},
				{
					Key:          "bool",
					DefaultValue: true,
				},
			},
			inputAttr: pcommon.NewMap(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("string", "default-string")
				res.PutInt("int", 123)
				res.PutDouble("double", 321.0)
				res.PutBool("bool", true)
				return res
			}(),
		},
		{
			name: "attributes from resource",
			config: []AttributeConfig{
				{
					Key: "string",
				},
				{
					Key: "int",
				},
				{
					Key: "double",
				},
				{
					Key: "bool",
				},
			},
			inputAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("string", "default-string")
				res.PutInt("int", 123)
				res.PutDouble("double", 321.0)
				res.PutBool("bool", true)
				return res
			}(),
			expectedAttr: func() pcommon.Map {
				res := pcommon.NewMap()
				res.PutStr("string", "default-string")
				res.PutInt("int", 123)
				res.PutDouble("double", 321.0)
				res.PutBool("bool", true)
				return res
			}(),
		},
	}

	for _, tt := range tests {
		spanMetricDefs := make(map[string]metricDef[ottlspan.TransformContext])
		spanMetricDefs[defaultMetricNameSpans] = metricDef[ottlspan.TransformContext]{
			desc:  defaultMetricDescSpans,
			attrs: tt.config,
		}

		t.Run(tt.name, func(t *testing.T) {
			spansCounter := newCounter(spanMetricDefs)
			err := spansCounter.update(t.Context(), pcommon.NewMap(), pcommon.NewMap(), tt.inputAttr, ottlspan.TransformContext{})
			require.NoError(t, err)
			require.NotNil(t, spansCounter)
			m := spansCounter.counts[defaultMetricNameSpans]
			expectKey := pdatautil.MapHash(tt.expectedAttr)
			attrCount, ok := m[expectKey]
			require.True(t, ok)
			require.NotNil(t, attrCount)
		})
	}
}
