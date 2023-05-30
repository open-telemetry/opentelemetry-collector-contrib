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
