// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
