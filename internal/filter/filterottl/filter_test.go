// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterottl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

func Test_NewBoolExprForSpan(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanBoolExpr, err := NewBoolExprForSpan(tt.conditions, StandardSpanFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, spanBoolExpr)
			result, err := spanBoolExpr.Eval(context.Background(), ottlspan.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForSpanWithOptions(t *testing.T) {
	_, err := NewBoolExprForSpanWithOptions(
		[]string{`span.name == "foo"`},
		StandardSpanFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottlspan.TransformContext]{ottlspan.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForSpanEvent(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanEventBoolExpr, err := NewBoolExprForSpanEvent(tt.conditions, StandardSpanEventFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, spanEventBoolExpr)
			result, err := spanEventBoolExpr.Eval(context.Background(), ottlspanevent.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForSpanEventWithOptions(t *testing.T) {
	_, err := NewBoolExprForSpanEventWithOptions(
		[]string{`spanevent.name == "foo"`},
		StandardSpanEventFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottlspanevent.TransformContext]{ottlspanevent.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForMetric(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricBoolExpr, err := NewBoolExprForMetric(tt.conditions, StandardMetricFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, metricBoolExpr)
			result, err := metricBoolExpr.Eval(context.Background(), ottlmetric.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForMetricWithOptions(t *testing.T) {
	_, err := NewBoolExprForMetricWithOptions(
		[]string{`metric.name == "foo"`},
		StandardMetricFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottlmetric.TransformContext]{ottlmetric.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForDataPoint(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataPointBoolExpr, err := NewBoolExprForDataPoint(tt.conditions, StandardDataPointFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, dataPointBoolExpr)
			result, err := dataPointBoolExpr.Eval(context.Background(), ottldatapoint.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForDataPointWithOptions(t *testing.T) {
	_, err := NewBoolExprForDataPointWithOptions(
		[]string{"datapoint.count > 0"},
		StandardDataPointFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottldatapoint.TransformContext]{ottldatapoint.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForLog(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logBoolExpr, err := NewBoolExprForLog(tt.conditions, StandardLogFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, logBoolExpr)
			result, err := logBoolExpr.Eval(context.Background(), ottllog.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForLogWithOptions(t *testing.T) {
	_, err := NewBoolExprForLogWithOptions(
		[]string{`log.body != ""`},
		StandardLogFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottllog.TransformContext]{ottllog.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForProfile(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profileBoolExpr, err := NewBoolExprForProfile(tt.conditions, StandardProfileFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, profileBoolExpr)
			result, err := profileBoolExpr.Eval(context.Background(), ottlprofile.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForProfileWithOptions(t *testing.T) {
	_, err := NewBoolExprForProfileWithOptions(
		[]string{`profile.duration_unix_nano > 0`},
		StandardProfileFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottlprofile.TransformContext]{ottlprofile.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForResource(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resBoolExpr, err := NewBoolExprForResource(tt.conditions, StandardResourceFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, resBoolExpr)
			result, err := resBoolExpr.Eval(context.Background(), ottlresource.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForResourceWithOptions(t *testing.T) {
	_, err := NewBoolExprForResourceWithOptions(
		[]string{`resource.dropped_attributes_count == 0`},
		StandardResourceFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottlresource.TransformContext]{ottlresource.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}

func Test_NewBoolExprForScope(t *testing.T) {
	tests := []struct {
		name           string
		conditions     []string
		expectedResult bool
	}{
		{
			name: "basic",
			conditions: []string{
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple conditions resulting true",
			conditions: []string{
				"false == true",
				"true == true",
			},
			expectedResult: true,
		},
		{
			name: "multiple conditions resulting false",
			conditions: []string{
				"false == true",
				"true == false",
			},
			expectedResult: false,
		},
		{
			name: "With Converter",
			conditions: []string{
				`IsMatch("test", "pass")`,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resBoolExpr, err := NewBoolExprForScope(tt.conditions, StandardScopeFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			assert.NoError(t, err)
			assert.NotNil(t, resBoolExpr)
			result, err := resBoolExpr.Eval(context.Background(), ottlscope.TransformContext{})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_NewBoolExprForScopeWithOptions(t *testing.T) {
	_, err := NewBoolExprForScopeWithOptions(
		[]string{`scope.name != ""`},
		StandardScopeFuncs(),
		ottl.PropagateError,
		componenttest.NewNopTelemetrySettings(),
		[]ottl.Option[ottlscope.TransformContext]{ottlscope.EnablePathContextNames()},
	)
	assert.NoError(t, err)
}
