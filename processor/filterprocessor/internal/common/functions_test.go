// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func Test_HasAttrKeyOnDatapoint(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		input    func() pmetric.Metric
		expected bool
	}{
		{
			name: "attribute present on sum datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on gauge datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on histogram datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on expo histogram datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on summary datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name: "attribute not present on sum datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySum().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on gauge datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyGauge().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on histogram datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyHistogram().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on expo histogram datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on summary datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySummary().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := hasAttributeKeyOnDatapoint(tt.key)
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), ottlmetric.NewTransformContext(tt.input(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_HasAttrOnDatapoint(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectedVal string
		input       func() pmetric.Metric
		expected    bool
	}{
		{
			name:        "attribute present on sum datapoint with value",
			key:         "test",
			expectedVal: "pass",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name:        "attribute present on gauge datapoint with value",
			key:         "test",
			expectedVal: "pass",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name:        "attribute present on histogram datapoint with value",
			key:         "test",
			expectedVal: "pass",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name:        "attribute present on expo histogram datapoint with value",
			key:         "test",
			expectedVal: "pass",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name:        "attribute present on summary datapoint with value",
			key:         "test",
			expectedVal: "pass",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on sum datapoint empty string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("test", "")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on gauge datapoint empty string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("test", "")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on histogram datapoint empty string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("test", "")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on expo histogram datapoint empty string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("test", "")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on summary datapoint empty string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("test", "")
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on sum datapoint not string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutBool("test", true)
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on gauge datapoint not string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutBool("test", true)
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on histogram datapoint not string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutBool("test", true)
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on expo histogram datapoint not string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutBool("test", true)
				return input
			},
			expected: true,
		},
		{
			name: "attribute present on summary datapoint not string",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutBool("test", true)
				return input
			},
			expected: true,
		},
		{
			name: "attribute not present on sum datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySum().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on gauge datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyGauge().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on histogram datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyHistogram().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on expo histogram datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
		{
			name: "attribute not present on summary datapoint",
			key:  "test",
			input: func() pmetric.Metric {
				input := pmetric.NewMetric()
				input.SetEmptySummary().DataPoints().AppendEmpty()
				return input
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := hasAttributeOnDatapoint(tt.key, tt.expectedVal)
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), ottlmetric.NewTransformContext(tt.input(), pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
