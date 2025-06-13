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
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func createConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}

func TestSpan_validateMatchesConfiguration_InvalidConfig(t *testing.T) {
	testcases := []struct {
		name        string
		property    *filterconfig.MatchProperties
		errorString string
	}{
		{
			name:        "empty_property",
			property:    &filterconfig.MatchProperties{},
			errorString: filterconfig.ErrMissingRequiredSpanField.Error(),
		},
		{
			name: "empty_service_span_names_and_attributes",
			property: &filterconfig.MatchProperties{
				Services: []string{},
			},
			errorString: filterconfig.ErrMissingRequiredSpanField.Error(),
		},
		{
			name: "log_properties",
			property: &filterconfig.MatchProperties{
				LogBodies: []string{"log"},
			},
			errorString: "log_bodies should not be specified for trace spans",
		},
		{
			name: "invalid_match_type",
			property: &filterconfig.MatchProperties{
				Config:   *createConfig("wrong_match_type"),
				Services: []string{"abc"},
			},
			errorString: "error creating service name filters: unrecognized match_type: 'wrong_match_type', valid types are: [regexp strict]",
		},
		{
			name: "missing_match_type",
			property: &filterconfig.MatchProperties{
				Services: []string{"abc"},
			},
			errorString: "error creating service name filters: unrecognized match_type: '', valid types are: [regexp strict]",
		},
		{
			name: "invalid_regexp_pattern_service",
			property: &filterconfig.MatchProperties{
				Config:   *createConfig(filterset.Regexp),
				Services: []string{"["},
			},
			errorString: "error creating service name filters: error parsing regexp: missing closing ]: `[`",
		},
		{
			name: "invalid_regexp_pattern_span",
			property: &filterconfig.MatchProperties{
				Config:    *createConfig(filterset.Regexp),
				SpanNames: []string{"["},
			},
			errorString: "error creating span name filters: error parsing regexp: missing closing ]: `[`",
		},
		{
			name: "invalid_strict_span_kind_match",
			property: &filterconfig.MatchProperties{
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
			output, err := newExpr(tc.property)
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

			val, err := expr.Eval(context.Background(), ottlspan.NewTransformContext(span, library, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans()))
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
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	emptySpan := ptrace.NewSpan()
	val, err := mp.Eval(context.Background(), ottlspan.NewTransformContext(emptySpan, pcommon.NewInstrumentationScope(), pcommon.NewResource(), ptrace.NewScopeSpans(), ptrace.NewResourceSpans()))
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
	resource.Attributes().PutStr(string(conventions.ServiceNameKey), "svcA")

	library := pcommon.NewInstrumentationScope()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := newExpr(tc.properties)
			require.NoError(t, err)
			assert.NotNil(t, mp)

			val, err := mp.Eval(context.Background(), ottlspan.NewTransformContext(span, library, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans()))
			require.NoError(t, err)
			assert.True(t, val)
		})
	}
}

func TestServiceNameForResource(t *testing.T) {
	td := testdata.GenerateTracesOneSpanNoResource()
	name := serviceNameForResource(td.ResourceSpans().At(0).Resource())
	require.Equal(t, "<nil-service-name>", name)

	td = testdata.GenerateTracesOneSpan()
	resource := td.ResourceSpans().At(0).Resource()
	name = serviceNameForResource(resource)
	require.Equal(t, "<nil-service-name>", name)
}

func Test_NewSkipExpr_With_Bridge(t *testing.T) {
	tests := []struct {
		name      string
		condition *filterconfig.MatchConfig
	}{
		// Service name
		{
			name: "single static service name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Services: []string{"svcA"},
				},
			},
		},
		{
			name: "multiple static service name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Services: []string{"svcB", "svcC"},
				},
			},
		},
		{
			name: "single regex service name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Services: []string{"svc.*"},
				},
			},
		},
		{
			name: "multiple regex service name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Services: []string{".*B", ".*C"},
				},
			},
		},
		{
			name: "single static service name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Services: []string{"svcA"},
				},
			},
		},
		{
			name: "multiple static service name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Services: []string{"svcB", "svcC"},
				},
			},
		},
		{
			name: "single regex service name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Services: []string{"svc.*"},
				},
			},
		},
		{
			name: "multiple regex service name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Services: []string{".*B", ".*C"},
				},
			},
		},

		// Span name
		{
			name: "single static span name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanNames: []string{"spanName"},
				},
			},
		},
		{
			name: "multiple static span name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanNames: []string{"foo", "bar"},
				},
			},
		},
		{
			name: "single regex span name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanNames: []string{"span.*"},
				},
			},
		},
		{
			name: "multiple regex span name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanNames: []string{"foo.*", "bar.*"},
				},
			},
		},
		{
			name: "single static span name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanNames: []string{"spanName"},
				},
			},
		},
		{
			name: "multiple static span name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanNames: []string{"foo", "bar"},
				},
			},
		},
		{
			name: "single regex span name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanNames: []string{"span.*"},
				},
			},
		},
		{
			name: "multiple regex span name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanNames: []string{"foo.*", "bar.*"},
				},
			},
		},

		// Span kind
		{
			name: "single static span kind include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanKinds: []string{"SPAN_KIND_CLIENT"},
				},
			},
		},
		{
			name: "multiple static span kind include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanKinds: []string{"SPAN_KIND_SERVER", "SPAN_KIND_PRODUCER"},
				},
			},
		},
		{
			name: "single regex span kind include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanKinds: []string{"SPAN_KIND_"},
				},
			},
		},
		{
			name: "multiple regex span kind include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanKinds: []string{"foo.*", "bar.*"},
				},
			},
		},
		{
			name: "single static span kind exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanKinds: []string{"SPAN_KIND_CLIENT"},
				},
			},
		},
		{
			name: "multiple static span kind exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanKinds: []string{"SPAN_KIND_SERVER", "SPAN_KIND_PRODUCER"},
				},
			},
		},
		{
			name: "single regex span kind exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanKinds: []string{"SPAN_KIND_"},
				},
			},
		},
		{
			name: "multiple regex span kind exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					SpanKinds: []string{"foo.*", "bar.*"},
				},
			},
		},

		// Scope name
		{
			name: "single static scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple static scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo",
						},
						{
							Name: "bar",
						},
					},
				},
			},
		},
		{
			name: "single regex scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope name include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo.*",
						},
						{
							Name: "bar.*",
						},
					},
				},
			},
		},
		{
			name: "single static scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple static scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo",
						},
						{
							Name: "bar",
						},
					},
				},
			},
		},
		{
			name: "single regex scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "scope",
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope name exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name: "foo.*",
						},
						{
							Name: "bar.*",
						},
					},
				},
			},
		},

		// Scope version
		{
			name: "single static scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.1.0"),
						},
					},
				},
			},
		},
		{
			name: "multiple static scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.0.0"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp(`1.1.0`),
						},
					},
				},
			},
		},
		{
			name: "single regex scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.*"),
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope version include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.*"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp("^1\\\\.1.*"),
						},
					},
				},
			},
		},
		{
			name: "single static scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.1.0"),
						},
					},
				},
			},
		},
		{
			name: "multiple static scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.0.0"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp(`1.1.0`),
						},
					},
				},
			},
		},
		{
			name: "single regex scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.*"),
						},
					},
				},
			},
		},
		{
			name: "multiple regex scope version exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("2.*"),
						},
						{
							Name:    "scope",
							Version: ottltest.Strp(`1\\.1.*`),
						},
					},
				},
			},
		},

		// attributes
		{
			name: "single static attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val1",
						},
					},
				},
			},
		},
		{
			name: "multiple static attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val2",
						},
						{
							Key:   "attr2",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "single regex attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val",
						},
						{
							Key:   "attr3",
							Value: "val.*",
						},
					},
				},
			},
		},
		{
			name: "single static attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val1",
						},
					},
				},
			},
		},
		{
			name: "multiple static attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val2",
						},
						{
							Key:   "attr2",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "single regex attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val",
						},
						{
							Key:   "attr3",
							Value: "val.*",
						},
					},
				},
			},
		},

		// resource attributes
		{
			name: "single static resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
			},
		},
		{
			name: "multiple static resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc2",
						},
						{
							Key:   "service.version",
							Value: "v1",
						},
					},
				},
			},
		},
		{
			name: "single regex resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex resource attribute include",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: ".*2",
						},
						{
							Key:   "service.name",
							Value: ".*3",
						},
					},
				},
			},
		},
		{
			name: "single static resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
			},
		},
		{
			name: "multiple static resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},

					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc2",
						},
						{
							Key:   "service.version",
							Value: "v1",
						},
					},
				},
			},
		},
		{
			name: "single regex resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svc.*",
						},
					},
				},
			},
		},
		{
			name: "multiple regex resource attribute exclude",
			condition: &filterconfig.MatchConfig{
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: ".*2",
						},
						{
							Key:   "service.name",
							Value: ".*3",
						},
					},
				},
			},
		},

		// complex
		{
			name: "complex",
			condition: &filterconfig.MatchConfig{
				Include: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Regexp,
					},
					Services: []string{"svcA"},
					Libraries: []filterconfig.InstrumentationLibrary{
						{
							Name:    "scope",
							Version: ottltest.Strp("0.1.0"),
						},
					},
					Resources: []filterconfig.Attribute{
						{
							Key:   "service.name",
							Value: "svcA",
						},
					},
				},
				Exclude: &filterconfig.MatchProperties{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					SpanNames: []string{"spanName"},
					SpanKinds: []string{"SPAN_KIND_CLIENT"},
					Attributes: []filterconfig.Attribute{
						{
							Key:   "attr1",
							Value: "val1",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetName("spanName")
			span.Attributes().PutStr("attr1", "val1")
			span.SetKind(ptrace.SpanKindClient)

			resource := pcommon.NewResource()
			resource.Attributes().PutStr("service.name", "svcA")

			scope := pcommon.NewInstrumentationScope()
			scope.SetName("scope")
			scope.SetVersion("0.1.0")

			tCtx := ottlspan.NewTransformContext(span, scope, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

			boolExpr, err := NewSkipExpr(tt.condition)
			require.NoError(t, err)
			expectedResult, err := boolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			ottlBoolExpr, err := filterottl.NewSpanSkipExprBridge(tt.condition)
			assert.NoError(t, err)
			ottlResult, err := ottlBoolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			assert.Equal(t, expectedResult, ottlResult)
		})
	}
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
		err := featuregate.GlobalRegistry().Set("filter.filterspan.useOTTLBridge", true)
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
		resource.Attributes().PutStr(string(conventions.ServiceNameKey), "svcA")

		scope := pcommon.NewInstrumentationScope()

		tCtx := ottlspan.NewTransformContext(span, scope, resource, ptrace.NewScopeSpans(), ptrace.NewResourceSpans())

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var skip bool
				skip, err = skipExpr.Eval(context.Background(), tCtx)
				assert.NoError(b, err)
				assert.Equal(b, tt.skip, skip)
			}
		})

		err = featuregate.GlobalRegistry().Set("filter.filterspan.useOTTLBridge", origVal)
		assert.NoError(b, err)
	}
}
