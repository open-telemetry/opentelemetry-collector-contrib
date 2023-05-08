// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterspan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

func createConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}

func TestSpan_validateMatchesConfiguration_InvalidConfig(t *testing.T) {
	testcases := []struct {
		name        string
		property    filterconfig.MatchProperties
		errorString string
	}{
		{
			name:        "empty_property",
			property:    filterconfig.MatchProperties{},
			errorString: filterconfig.ErrMissingRequiredField.Error(),
		},
		{
			name: "empty_service_span_names_and_attributes",
			property: filterconfig.MatchProperties{
				Services: []string{},
			},
			errorString: filterconfig.ErrMissingRequiredField.Error(),
		},
		{
			name: "log_properties",
			property: filterconfig.MatchProperties{
				LogBodies: []string{"log"},
			},
			errorString: "log_bodies should not be specified for trace spans",
		},
		{
			name: "invalid_match_type",
			property: filterconfig.MatchProperties{
				Config:   *createConfig("wrong_match_type"),
				Services: []string{"abc"},
			},
			errorString: "error creating service name filters: unrecognized match_type: 'wrong_match_type', valid types are: [regexp strict]",
		},
		{
			name: "missing_match_type",
			property: filterconfig.MatchProperties{
				Services: []string{"abc"},
			},
			errorString: "error creating service name filters: unrecognized match_type: '', valid types are: [regexp strict]",
		},
		{
			name: "invalid_regexp_pattern_service",
			property: filterconfig.MatchProperties{
				Config:   *createConfig(filterset.Regexp),
				Services: []string{"["},
			},
			errorString: "error creating service name filters: error parsing regexp: missing closing ]: `[`",
		},
		{
			name: "invalid_regexp_pattern_span",
			property: filterconfig.MatchProperties{
				Config:    *createConfig(filterset.Regexp),
				SpanNames: []string{"["},
			},
			errorString: "error creating span name filters: error parsing regexp: missing closing ]: `[`",
		},
		{
			name: "invalid_strict_span_kind_match",
			property: filterconfig.MatchProperties{
				Config: *createConfig(filterset.Strict),
				SpanKinds: []string{
					"test_invalid_span_kind",
				},
				Attributes: []filterconfig.Attribute{},
			},
			errorString: "span_kinds string must match one of the standard span kinds when match_type=strict: [     SPAN_KIND_CLIENT SPAN_KIND_CONSUMER SPAN_KIND_INTERNAL SPAN_KIND_PRODUCER SPAN_KIND_SERVER]",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := newExpr(&tc.property)
			assert.Nil(t, output)
			assert.EqualError(t, err, tc.errorString)
		})
	}
}

func TestSpan_Matching_False(t *testing.T) {
	testcases := []struct {
		name       string
		properties *filterconfig.MatchProperties
	}{
		{
			name: "service_name_doesnt_match_regexp",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				Services:   []string{"svcA"},
				Attributes: []filterconfig.Attribute{},
			},
		},

		{
			name: "service_name_doesnt_match_strict",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Strict),
				Services:   []string{"svcA"},
				Attributes: []filterconfig.Attribute{},
			},
		},

		{
			name: "span_name_doesnt_match",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				SpanNames:  []string{"spanNo.*Name"},
				Attributes: []filterconfig.Attribute{},
			},
		},

		{
			name: "span_name_doesnt_match_any",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				SpanNames: []string{
					"spanNo.*Name",
					"non-matching?pattern",
					"regular string",
				},
				Attributes: []filterconfig.Attribute{},
			},
		},
		{
			name: "span_kind_doesnt_match_regexp",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				Attributes: []filterconfig.Attribute{},
				SpanKinds:  []string{traceutil.SpanKindStr(ptrace.SpanKindProducer)},
			},
		},
		{
			name: "span_kind_doesnt_match_strict",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Strict),
				Attributes: []filterconfig.Attribute{},
				SpanKinds:  []string{traceutil.SpanKindStr(ptrace.SpanKindProducer)},
			},
		},
	}

	span := ptrace.NewSpan()
	span.SetName("spanName")
	library := pcommon.NewInstrumentationScope()
	resource := pcommon.NewResource()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := newExpr(tc.properties)
			require.NoError(t, err)
			assert.NotNil(t, expr)

			val, err := expr.Eval(context.Background(), ottlspan.NewTransformContext(span, library, resource))
			require.NoError(t, err)
			assert.False(t, val)
		})
	}
}

func TestSpan_MissingServiceName(t *testing.T) {
	cfg := &filterconfig.MatchProperties{
		Config:   *createConfig(filterset.Regexp),
		Services: []string{"svcA"},
	}

	mp, err := newExpr(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, mp)

	emptySpan := ptrace.NewSpan()
	val, err := mp.Eval(context.Background(), ottlspan.NewTransformContext(emptySpan, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
	require.NoError(t, err)
	assert.False(t, val)
}

func TestSpan_Matching_True(t *testing.T) {
	testcases := []struct {
		name       string
		properties *filterconfig.MatchProperties
	}{
		{
			name: "service_name_match_regexp",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				Services:   []string{"svcA"},
				Attributes: []filterconfig.Attribute{},
			},
		},
		{
			name: "service_name_match_strict",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Strict),
				Services:   []string{"svcA"},
				Attributes: []filterconfig.Attribute{},
			},
		},
		{
			name: "span_name_match",
			properties: &filterconfig.MatchProperties{
				Config:     *createConfig(filterset.Regexp),
				SpanNames:  []string{"span.*"},
				Attributes: []filterconfig.Attribute{},
			},
		},
		{
			name: "span_name_second_match",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				SpanNames: []string{
					"wrong.*pattern",
					"span.*",
					"yet another?pattern",
					"regularstring",
				},
				Attributes: []filterconfig.Attribute{},
			},
		},
		{
			name: "span_kind_match_strict",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Strict),
				SpanKinds: []string{
					traceutil.SpanKindStr(ptrace.SpanKindClient),
				},
				Attributes: []filterconfig.Attribute{},
			},
		},
		{
			name: "span_kind_match_regexp",
			properties: &filterconfig.MatchProperties{
				Config: *createConfig(filterset.Regexp),
				SpanKinds: []string{
					"CLIENT",
				},
				Attributes: []filterconfig.Attribute{},
			},
		},
	}

	span := ptrace.NewSpan()
	span.SetName("spanName")

	span.Attributes().PutStr("keyString", "arithmetic")
	span.Attributes().PutInt("keyInt", 123)
	span.Attributes().PutDouble("keyDouble", 3245.6)
	span.Attributes().PutBool("keyBool", true)
	span.Attributes().PutStr("keyExists", "present")
	span.SetKind(ptrace.SpanKindClient)
	assert.NotNil(t, span)

	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventions.AttributeServiceName, "svcA")

	library := pcommon.NewInstrumentationScope()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := newExpr(tc.properties)
			require.NoError(t, err)
			assert.NotNil(t, mp)

			val, err := mp.Eval(context.Background(), ottlspan.NewTransformContext(span, library, resource))
			require.NoError(t, err)
			assert.True(t, val)
		})
	}
}

func TestServiceNameForResource(t *testing.T) {
	td := testdata.GenerateTracesOneSpanNoResource()
	name := serviceNameForResource(td.ResourceSpans().At(0).Resource())
	require.Equal(t, name, "<nil-service-name>")

	td = testdata.GenerateTracesOneSpan()
	resource := td.ResourceSpans().At(0).Resource()
	name = serviceNameForResource(resource)
	require.Equal(t, name, "<nil-service-name>")

}

func BenchmarkFilterspan_NewSkipExpr(b *testing.B) {
	testCases := []struct {
		name string
		mc   *filterconfig.MatchConfig
		skip bool
	}{
		{
			name: "service_name_match_regexp",
			mc: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config:   *createConfig(filterset.Regexp),
					Services: []string{"svcA"},
				},
			},
			skip: false,
		},
		{
			name: "service_name_match_static",
			mc: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config:   *createConfig(filterset.Strict),
					Services: []string{"svcA"},
				},
			},
			skip: false,
		},
	}

	for _, tt := range testCases {
		origVal := useOTTLBridge.IsEnabled()
		err := featuregate.GlobalRegistry().Set("useOTTLBridge", true)
		assert.NoError(b, err)

		skipExpr, err := NewSkipExpr(tt.mc)
		assert.NoError(b, err)

		span := ptrace.NewSpan()
		span.SetName("spanName")

		span.Attributes().PutStr("keyString", "arithmetic")
		span.Attributes().PutInt("keyInt", 123)
		span.Attributes().PutDouble("keyDouble", 3245.6)
		span.Attributes().PutBool("keyBool", true)
		span.Attributes().PutStr("keyExists", "present")
		span.SetKind(ptrace.SpanKindClient)

		resource := pcommon.NewResource()
		resource.Attributes().PutStr(conventions.AttributeServiceName, "svcA")

		scope := pcommon.NewInstrumentationScope()

		tCtx := ottlspan.NewTransformContext(span, scope, resource)

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var skip bool
				skip, err = skipExpr.Eval(context.Background(), tCtx)
				assert.NoError(b, err)
				assert.Equal(b, tt.skip, skip)
			}
		})

		err = featuregate.GlobalRegistry().Set("useOTTLBridge", origVal)
		assert.NoError(b, err)
	}
}
