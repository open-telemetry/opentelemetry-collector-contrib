// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprofile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := pprofile.NewProfile()
			span.SetDuration(100)

			resource := pcommon.NewResource()
			resource.Attributes().PutStr("service.name", "svcA")

			scope := pcommon.NewInstrumentationScope()
			scope.SetName("scope")
			scope.SetVersion("0.1.0")

			tCtx := ottlprofile.NewTransformContext(span, scope, resource, pprofile.NewScopeProfiles(), pprofile.NewResourceProfiles())

			boolExpr, err := NewSkipExpr(tt.condition)
			require.NoError(t, err)
			expectedResult, err := boolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			ottlBoolExpr, err := filterottl.NewProfileSkipExprBridge(tt.condition)
			assert.NoError(t, err)
			ottlResult, err := ottlBoolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			assert.Equal(t, expectedResult, ottlResult)
		})
	}
}
