// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestAggregation(t *testing.T) {
	testCases := []struct {
		name         string
		input        map[string]pcommon.Value
		expected     map[string]pcommon.Value
		aggregations []*aggregation
	}{
		{
			name: "three values one key",
			input: map[string]pcommon.Value{
				"pod_first":  pcommon.NewValueStr("first"),
				"pod_second": pcommon.NewValueStr("second"),
				"pod_third":  pcommon.NewValueStr("third"),
			},
			expected: map[string]pcommon.Value{
				"pods": mapToPcommonValue(map[string]pcommon.Value{
					"first":  pcommon.NewValueStr("first"),
					"second": pcommon.NewValueStr("second"),
					"third":  pcommon.NewValueStr("third"),
				}),
			},
			aggregations: []*aggregation{
				{
					attribute: "pods",
					prefixes:  []string{"pod_"},
				},
			},
		},
		{
			name: "six values two keys",
			input: map[string]pcommon.Value{
				"pod_first":                pcommon.NewValueStr("first"),
				"pod_second":               pcommon.NewValueStr("second"),
				"pod_third":                pcommon.NewValueStr("third"),
				"sono_ichi":                pcommon.NewValueInt(1),
				"sono_ni":                  pcommon.NewValueInt(2),
				"a totally unrelevant key": pcommon.NewValueBool(true),
			},
			expected: map[string]pcommon.Value{
				"pods": mapToPcommonValue(map[string]pcommon.Value{
					"first":  pcommon.NewValueStr("first"),
					"second": pcommon.NewValueStr("second"),
					"third":  pcommon.NewValueStr("third"),
				}),
				"counts": mapToPcommonValue(map[string]pcommon.Value{
					"ichi": pcommon.NewValueInt(1),
					"ni":   pcommon.NewValueInt(2),
				}),
				"a totally unrelevant key": pcommon.NewValueBool(true),
			},
			aggregations: []*aggregation{
				{
					attribute: "pods",
					prefixes:  []string{"pod_"},
				},
				{
					attribute: "counts",
					prefixes:  []string{"sono_"},
				},
			},
		},
		{
			name: "three prefixes, one key",
			input: map[string]pcommon.Value{
				"A_12": pcommon.NewValueStr("A12"),
				"A_23": pcommon.NewValueStr("A23"),
				"C_2":  pcommon.NewValueStr("C2"),
				"B_3":  pcommon.NewValueStr("B3"),
				"C_88": pcommon.NewValueStr("C88"),
				"B_53": pcommon.NewValueStr("B53"),
			},
			expected: map[string]pcommon.Value{
				"id": mapToPcommonValue(map[string]pcommon.Value{
					"2":  pcommon.NewValueStr("C2"),
					"3":  pcommon.NewValueStr("B3"),
					"12": pcommon.NewValueStr("A12"),
					"23": pcommon.NewValueStr("A23"),
					"53": pcommon.NewValueStr("B53"),
					"88": pcommon.NewValueStr("C88"),
				}),
			},
			aggregations: []*aggregation{
				{
					attribute: "id",
					prefixes:  []string{"B_", "A_", "C_"},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			processor := aggregateAttributesProcessor{
				aggregations: testCase.aggregations,
			}

			attrs := mapToPcommonMap(testCase.input)

			err := processor.processAttributes(attrs)
			require.NoError(t, err)

			expected := mapToPcommonMap(testCase.expected)

			require.Equal(t, expected.AsRaw(), attrs.AsRaw())
		})
	}
}

func TestMetrics(t *testing.T) {
	aggregations := []*aggregation{{
		attribute: "a",
		prefixes:  []string{"b_"},
	}}
	testCases := []struct {
		name         string
		createMetric func() pmetric.Metric
		test         func(pmetric.Metric)
	}{
		{
			name:         "empty",
			createMetric: pmetric.NewMetric,
			test: func(m pmetric.Metric) {
				require.Equal(t, m.Type(), pmetric.MetricTypeEmpty)
			},
		},
		{
			name: "sum",
			createMetric: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySum()
				s.DataPoints().AppendEmpty().Attributes().PutStr("b_c", "x")

				return m
			},
			test: func(m pmetric.Metric) {
				s := pmetric.NewMetric().SetEmptySum()
				s.DataPoints().AppendEmpty().Attributes().PutEmptyMap("a").PutStr("c", "x")

				require.Equal(t, m.Type(), pmetric.MetricTypeSum)
				require.Equal(t, s.DataPoints().At(0).Attributes().AsRaw(), m.Sum().DataPoints().At(0).Attributes().AsRaw())
			},
		},
		{
			name: "gauge",
			createMetric: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyGauge()
				s.DataPoints().AppendEmpty().Attributes().PutStr("b_c", "x")

				return m
			},
			test: func(m pmetric.Metric) {
				s := pmetric.NewMetric().SetEmptyGauge()
				s.DataPoints().AppendEmpty().Attributes().PutEmptyMap("a").PutStr("c", "x")

				require.Equal(t, m.Type(), pmetric.MetricTypeGauge)
				require.Equal(t, s.DataPoints().At(0).Attributes().AsRaw(), m.Gauge().DataPoints().At(0).Attributes().AsRaw())
			},
		},
		{
			name: "histogram",
			createMetric: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyHistogram()
				s.DataPoints().AppendEmpty().Attributes().PutStr("b_c", "x")

				return m
			},
			test: func(m pmetric.Metric) {
				s := pmetric.NewMetric().SetEmptyHistogram()
				s.DataPoints().AppendEmpty().Attributes().PutEmptyMap("a").PutStr("c", "x")

				require.Equal(t, m.Type(), pmetric.MetricTypeHistogram)
				require.Equal(t, s.DataPoints().At(0).Attributes().AsRaw(), m.Histogram().DataPoints().At(0).Attributes().AsRaw())
			},
		},
		{
			name: "exponential histogram",
			createMetric: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptyExponentialHistogram()
				s.DataPoints().AppendEmpty().Attributes().PutStr("b_c", "x")

				return m
			},
			test: func(m pmetric.Metric) {
				s := pmetric.NewMetric().SetEmptyExponentialHistogram()
				s.DataPoints().AppendEmpty().Attributes().PutEmptyMap("a").PutStr("c", "x")

				require.Equal(t, m.Type(), pmetric.MetricTypeExponentialHistogram)
				require.Equal(t, s.DataPoints().At(0).Attributes().AsRaw(), m.ExponentialHistogram().DataPoints().At(0).Attributes().AsRaw())
			},
		},
		{
			name: "summary",
			createMetric: func() pmetric.Metric {
				m := pmetric.NewMetric()
				s := m.SetEmptySummary()
				s.DataPoints().AppendEmpty().Attributes().PutStr("b_c", "x")

				return m
			},
			test: func(m pmetric.Metric) {
				s := pmetric.NewMetric().SetEmptySummary()
				s.DataPoints().AppendEmpty().Attributes().PutEmptyMap("a").PutStr("c", "x")

				require.Equal(t, m.Type(), pmetric.MetricTypeSummary)
				require.Equal(t, s.DataPoints().At(0).Attributes().AsRaw(), m.Summary().DataPoints().At(0).Attributes().AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			processor := aggregateAttributesProcessor{
				aggregations: aggregations,
			}

			metric := testCase.createMetric()
			err := processMetricLevelAttributes(&processor, metric)
			require.NoError(t, err)

			testCase.test(metric)
		})
	}
}
